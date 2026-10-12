package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// The two stores this app uses, and the one rule about the order they are written in.
//
// S3 holds the bytes and Postgres holds everything else: who the file is called, how big it
// is, when it arrived and which S3 key it is under. Neither knows about the other; this
// file is the only place that knows about both.
//
// **Write S3 first, then the row. Delete the row first, then S3.** That order is the whole
// of the reliability story here and it is worth understanding before you change anything:
//
//   - If the upload to S3 works and the insert then fails, you have an object nobody points
//     at. Nothing is broken; it is a few bytes nobody can see, and a tidy-up job could
//     sweep it later.
//   - If you wrote the row first and the upload then failed, the page would list a file that
//     cannot be opened. Somebody clicks it and gets an error, and there is no way to tell
//     that row from a good one.
//
// The first is invisible, the second is a broken page. So always write the thing that can
// be orphaned harmlessly first, and delete in the opposite order for the same reason.

// bucket is the bucket this app keeps its uploads in.
//
// **The app makes its own bucket and is not given one.** The cloud stack in your session
// starts completely empty, exactly as a real AWS account you just opened would, so
// `CreateBucket` is your code's job, and it is one of the things worth learning. A name
// must be lower case and DNS-safe.
var bucket = envOr("BUCKET_NAME", "uploads")

func envOr(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

// The two clients, made once by `setUp`.
var (
	s3Client *s3.Client
	db       *pgxpool.Pool
)

// setUp makes the clients, the bucket and the table, if they are not there already.
//
// **Run on every start and safe to run again**, which is what `IF NOT EXISTS` and catching
// "you already own this bucket" are for. An app that only works the first time it starts is
// an app that breaks the moment it restarts.
func setUp(ctx context.Context) error {
	// **`UsePathStyle` is the one setting you must not miss**, and leaving it out is the
	// mistake almost everybody makes first. By default the SDK addresses a bucket as part of
	// the hostname, `uploads.cloud`, and inside your session there is no such name, so
	// every call fails with a DNS error that says nothing about buckets. Path style asks
	// for `http://cloud:4566/uploads/...` instead, which is a name that exists.
	//
	// Everything else comes from the environment your session already set:
	// `AWS_ENDPOINT_URL`, `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY` and `AWS_REGION`. The
	// SDK reads the credentials by itself, which is why they are not mentioned below.
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(envOr("AWS_REGION", "us-east-1")))
	if err != nil {
		return err
	}
	s3Client = s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.UsePathStyle = true
		if endpoint := os.Getenv("AWS_ENDPOINT_URL"); endpoint != "" {
			o.BaseEndpoint = aws.String(endpoint)
		}
		// Only add a checksum when S3 insists on one: some S3-shaped stand-ins do not
		// understand the extra checksum headers newer SDKs send by default.
		o.RequestChecksumCalculation = aws.RequestChecksumCalculationWhenRequired
		o.ResponseChecksumValidation = aws.ResponseChecksumValidationWhenRequired
	})

	// The database your session started, at the name `db`. `DATABASE_URL` is set for you.
	db, err = pgxpool.New(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		return err
	}

	if _, err := s3Client.CreateBucket(ctx, &s3.CreateBucketInput{Bucket: aws.String(bucket)}); err != nil {
		// Already there, and made by us. Any other failure is real and should stop the app,
		// because an app that cannot reach its store has nothing useful to do.
		var owned *types.BucketAlreadyOwnedByYou
		var exists *types.BucketAlreadyExists
		if !errors.As(err, &owned) && !errors.As(err, &exists) {
			return err
		}
	} else {
		log.Printf("made the bucket %s", bucket)
	}

	if _, err := db.Exec(ctx, `
    CREATE TABLE IF NOT EXISTS files (
      id          bigserial PRIMARY KEY,
      -- **The key, which is where the bytes are.** A uuid rather than the name somebody
      -- uploaded: two people may upload "notes.txt", and a key built from a name is a
      -- file that quietly replaces another one.
      s3_key      text NOT NULL,
      -- What to call it on the page and in the download. Kept apart from the key for the
      -- reason above.
      name        text NOT NULL,
      size_bytes  bigint NOT NULL,
      uploaded_at timestamptz NOT NULL DEFAULT now()
    )
  `); err != nil {
		return err
	}
	log.Println("the files table is ready")
	return nil
}

// newKey makes a random (version 4) uuid for an S3 key, from the operating system's
// random numbers.
func newKey() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // the standard variant
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}

// listFiles gives every file, newest first.
func listFiles(ctx context.Context) ([]fileRow, error) {
	rows, err := db.Query(ctx,
		`SELECT id, name, size_bytes, uploaded_at FROM files ORDER BY uploaded_at DESC, id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var files []fileRow
	for rows.Next() {
		var f fileRow
		if err := rows.Scan(&f.ID, &f.Name, &f.SizeBytes, &f.UploadedAt); err != nil {
			return nil, err
		}
		files = append(files, f)
	}
	return files, rows.Err()
}

// addFile keeps one file: the bytes in S3, then the row in Postgres.
//
// See this file's own note on why that order and not the other.
func addFile(ctx context.Context, key, name string, data []byte, kind string) error {
	if kind == "" {
		kind = "application/octet-stream"
	}
	if _, err := s3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(data),
		ContentType: aws.String(kind),
	}); err != nil {
		return err
	}

	_, err := db.Exec(ctx,
		`INSERT INTO files (s3_key, name, size_bytes) VALUES ($1, $2, $3)`,
		key, name, int64(len(data)))
	return err
}

// fileByID gives one file's row, or nil when there is none.
//
// The id arrives from the address as text. `$1::text::bigint` hands it to Postgres as text
// and lets Postgres do the conversion, so an address like `/files/abc` is an error
// message from the database rather than a crash here.
func fileByID(ctx context.Context, id string) (*fileRow, error) {
	var f fileRow
	err := db.QueryRow(ctx,
		`SELECT id, s3_key, name, size_bytes FROM files WHERE id = $1::text::bigint`, id).
		Scan(&f.ID, &f.S3Key, &f.Name, &f.SizeBytes)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &f, nil
}

// readFile gives the bytes back out of S3, as a stream. The caller closes it.
func readFile(ctx context.Context, key string) (io.ReadCloser, error) {
	got, err := s3Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, err
	}
	return got.Body, nil
}

// removeFile forgets one file: the row first, then the bytes.
//
// **The opposite order from writing, and for the same reason.** Delete the bytes first and
// a failure between the two leaves a row pointing at nothing, which is a broken download.
// Delete the row first and the worst case is an object nobody points at.
func removeFile(ctx context.Context, id string) (bool, error) {
	file, err := fileByID(ctx, id)
	if err != nil || file == nil {
		return false, err
	}

	if _, err := db.Exec(ctx, `DELETE FROM files WHERE id = $1::text::bigint`, id); err != nil {
		return false, err
	}
	if _, err := s3Client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(file.S3Key),
	}); err != nil {
		return false, err
	}
	return true, nil
}
