# File uploads in Go: S3 for the bytes, Postgres for everything else

A sample app for the [MitsuruTech Learning Platform](https://mitsurutech.co.uk). Upload a
file; S3 keeps it; Postgres remembers where it went and what it is called. The table on the
page comes out of Postgres, and every download comes out of S3.

It is deliberately small, four routes and three files, so you can read the whole thing
before you change any of it.

```
main.go    the web: four routes and nothing else
store.go   the two stores, and the order they are written in
page.go    the HTML
```

## Deploying it here

Open **Deploy** on the platform, pick this repository, and start a **database** and the
**cloud stack** alongside it. Everything the app needs is then in its environment already;
there is nothing to configure.

| Variable | Set by | What it is |
| --- | --- | --- |
| `DATABASE_URL` | your session | the Postgres server running as `db` |
| `AWS_ENDPOINT_URL` | your session | the cloud stack running as `cloud` |
| `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, `AWS_REGION` | your session | credentials for it |
| `PORT` | the platform | the port to listen on |
| `BUCKET_NAME` | you, optionally | defaults to `uploads` |

The platform builds it with `go build -o app .` and starts it with `./app`.

## The three things worth reading

**1. `o.UsePathStyle = true`, in `store.go`.** This is the setting everybody misses first. By
default the AWS SDK puts the bucket in the hostname, `uploads.cloud`, and there is no such
name on your session's network, so every call fails with a DNS error that says nothing about
buckets. Path style asks for `http://cloud:4566/uploads/...` instead.

**2. The app makes its own bucket.** Your cloud stack starts completely empty, exactly as a
real AWS account you just opened would. `CreateBucket` is your code's job, it runs on every
start, and it accepts "you already own this" so a restart is not a crash.

**3. Write S3 first, then the row. Delete the row first, then S3.**

- Upload the bytes, then insert the row. If the insert fails you have an object nobody
  points at: invisible, harmless, sweepable.
- Write the row first and a failed upload leaves a file on the page that cannot be opened,
  and no way to tell that row from a good one.
- Deleting runs the other way for the same reason.

One of those failures is a few wasted bytes. The other is a broken page. Always let the
thing that can be orphaned harmlessly go first.

## Two safety habits, taken seriously in a sample on purpose

**The S3 key is a uuid, never the filename.** Two people upload `notes.txt` and one must not
overwrite the other; and a name that came from a browser is not a thing to build a path from
(`../../etc/passwd` is a perfectly valid filename as far as a form is concerned). The real
name is a column, which is where it is safe.

**Downloads are always `attachment`, always `application/octet-stream`, always `nosniff`.**
Anybody can upload anything. An HTML file served inline would run as a page on your app's own
address, with whatever that page can reach; saved to disk it is inert.

And every filename is escaped on the way back onto the page: a file called
`<img src=x onerror=alert(1)>` is a perfectly good filename and must not become markup.

## Running it on your own machine

You need Go, a Postgres and something S3-shaped. With Docker:

```bash
docker run -d -p 5432:5432 -e POSTGRES_PASSWORD=dev --name pg postgres:16
docker run -d -p 4566:4566 --name cloud localstack/localstack

export DATABASE_URL=postgres://postgres:dev@localhost:5432/postgres
export AWS_ENDPOINT_URL=http://localhost:4566
export AWS_ACCESS_KEY_ID=test AWS_SECRET_ACCESS_KEY=test AWS_REGION=us-east-1

go build -o app . && ./app
```

Then open <http://localhost:8080>.

## Things to try next

- Stream a large upload straight to S3 instead of holding it in memory (`main.go` says
  where).
- Add a download count, in DynamoDB: your cloud stack emulates that too.
- Put the files behind a login, so the table is per person.
- Show an image preview, using a pre-signed URL rather than proxying the bytes.

## Licence

MIT. Fork it, break it, keep the pieces.

## About

Made by [Mitsuru Technologies](https://mitsurutech.co.uk/learning), who build the
MitsuruTech Learning Platform.

The same sample in other languages: [Node](https://github.com/mitsurutech/uploads-node),
[Python](https://github.com/mitsurutech/uploads-python) and
[Java](https://github.com/mitsurutech/uploads-java).
