package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
)

// A file upload, end to end: the browser, S3 and Postgres.
//
// Four routes and nothing else, so the whole thing can be read in one sitting:
//
//	GET  /                   the page: a form, and a table of what has been uploaded
//	POST /upload             keep one file
//	GET  /files/{id}         send one back
//	POST /files/{id}/delete  forget one
//
// Everything about *how* the two stores work is in `store.go`. This file is about the web.

// maxUpload is **a limit somebody chose, not a rule**: ten megabytes.
//
// **The file is kept in memory and not on disk**, which is right here and would be wrong
// for a bigger one. Your app's filesystem does not survive a restart, so writing uploads
// there would lose them; and the point of this sample is that S3 is where bytes live. A
// file larger than this should be streamed straight to S3 rather than held whole.
const maxUpload = 10 * 1024 * 1024

// errTooLarge is what an upload over the limit ends as: the same words as the Node
// sample, shown on the same error page.
var errTooLarge = errors.New("File too large")

// fail says what went wrong, rather than swallowing it: a blank page teaches nobody
// anything. It is the one place an unexpected error becomes a page.
func fail(w http.ResponseWriter, why error) {
	log.Println(why)
	sendPage(w, http.StatusInternalServerError, page(nil, "Something went wrong: "+why.Error()))
}

func sendPage(w http.ResponseWriter, status int, body string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	io.WriteString(w, body)
}

// upload is the one file a form carried.
type upload struct {
	name  string
	bytes []byte
	kind  string
}

// readUpload finds the part of the form called `file` and reads it, never more than
// `maxUpload` bytes of it. It answers nil when there is no file: not a multipart form, no
// part with that name, or a browser that sent the form with nothing chosen (an empty
// filename).
func readUpload(r *http.Request) (*upload, error) {
	reader, err := r.MultipartReader()
	if err != nil {
		return nil, nil // not a form with files in it, so no file
	}
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			return nil, nil
		}
		if err != nil {
			return nil, err
		}
		if part.FormName() != "file" || part.FileName() == "" {
			continue
		}

		// Read one byte more than the limit, so "exactly 10 MB" is allowed and "a byte
		// over" is told apart from it.
		bytes, err := io.ReadAll(io.LimitReader(part, maxUpload+1))
		if err != nil {
			return nil, err
		}
		if len(bytes) > maxUpload {
			return nil, errTooLarge
		}
		kind := part.Header.Get("Content-Type")
		if kind == "" {
			kind = "text/plain"
		}
		return &upload{name: part.FileName(), bytes: bytes, kind: kind}, nil
	}
}

func main() {
	ctx := context.Background()

	// **Stop rather than serve a broken app.** An app that starts without its bucket or its
	// table answers every request with the same error, and the logs are the only place that
	// says why; failing at the start puts the reason where somebody is already looking.
	if err := setUp(ctx); err != nil {
		log.Println("Could not set up the stores:", err)
		os.Exit(1)
	}

	mux := http.NewServeMux()

	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		files, err := listFiles(r.Context())
		if err != nil {
			fail(w, err)
			return
		}
		sendPage(w, http.StatusOK, page(files, ""))
	})

	mux.HandleFunc("POST /upload", func(w http.ResponseWriter, r *http.Request) {
		file, err := readUpload(r)
		if err != nil {
			fail(w, err)
			return
		}
		if file == nil {
			files, err := listFiles(r.Context())
			if err != nil {
				fail(w, err)
				return
			}
			sendPage(w, http.StatusBadRequest, page(files, "Choose a file first."))
			return
		}

		// **A uuid for the key, never the name somebody typed.** Two people uploading
		// `notes.txt` must not overwrite each other, and a name that arrived from a browser
		// is not a thing to build a path out of: `../../etc/passwd` is a perfectly good
		// filename as far as a form is concerned. The real name is a column, which is where
		// it is safe.
		key, err := newKey()
		if err != nil {
			fail(w, err)
			return
		}
		if err := addFile(r.Context(), key, file.name, file.bytes, file.kind); err != nil {
			fail(w, err)
			return
		}

		// **See the other page, not this one.** A refresh after a POST re-posts it, which is
		// how somebody uploads the same file three times by pressing F5.
		http.Redirect(w, r, "/", http.StatusSeeOther)
	})

	mux.HandleFunc("GET /files/{id}", func(w http.ResponseWriter, r *http.Request) {
		file, err := fileByID(r.Context(), r.PathValue("id"))
		if err != nil {
			fail(w, err)
			return
		}
		if file == nil {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.WriteHeader(http.StatusNotFound)
			io.WriteString(w, "There is no such file.")
			return
		}

		body, err := readFile(r.Context(), file.S3Key)
		if err != nil {
			fail(w, err)
			return
		}
		defer body.Close()

		// **Always an attachment, and always `octet-stream`.** Anybody can upload anything
		// here, and an HTML file served inline would run as a page on this app's own
		// address. Saved to disk it is inert. `nosniff` stops a browser guessing a type of
		// its own and undoing the first two.
		quoteless := strings.NewReplacer(`"`, "", `\`, "").Replace(file.Name)
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, quoteless))
		io.Copy(w, body)
	})

	mux.HandleFunc("POST /files/{id}/delete", func(w http.ResponseWriter, r *http.Request) {
		if _, err := removeFile(r.Context(), r.PathValue("id")); err != nil {
			fail(w, err)
			return
		}
		http.Redirect(w, r, "/", http.StatusSeeOther)
	})

	// **`PORT` comes from the platform and must not be hard-coded.** Your app is one service
	// on a private network and the door in front of it dials the port it was told about.
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("listening on %s", port)
	if err := http.ListenAndServe("0.0.0.0:"+port, mux); err != nil {
		log.Println(err)
		os.Exit(1)
	}
}
