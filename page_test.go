package main

import (
	"os"
	"testing"
	"time"
)

// The page must match, byte for byte, what the Node sample draws for the same inputs.
// `testdata/*.html` were made with `TZ=UTC node ref-render.mjs two|empty|trouble`.

func golden(t *testing.T, name, got string) {
	t.Helper()
	want, err := os.ReadFile("testdata/" + name + ".html")
	if err != nil {
		t.Fatal(err)
	}
	if got != string(want) {
		t.Fatalf("%s differs from the reference page (got %d bytes, want %d)", name, len(got), len(want))
	}
}

func TestTwoFiles(t *testing.T) {
	files := []fileRow{
		{ID: 2, Name: "<img src=x onerror=alert(1)>.png", SizeBytes: 1536,
			UploadedAt: time.Date(2026, 10, 12, 14, 5, 9, 0, time.UTC)},
		{ID: 1, Name: `notes "quoted" & more.txt`, SizeBytes: 12,
			UploadedAt: time.Date(2026, 10, 11, 9, 1, 2, 0, time.UTC)},
	}
	golden(t, "two", page(files, ""))
}

func TestEmpty(t *testing.T) { golden(t, "empty", page(nil, "")) }

func TestTrouble(t *testing.T) {
	golden(t, "trouble", page(nil, "Something went wrong: <b>x</b>"))
}

func TestSmallPieces(t *testing.T) {
	for in, want := range map[int64]string{0: "0 B", 1023: "1023 B", 1024: "1.0 KB", 1536: "1.5 KB", 5 * 1024 * 1024: "5.0 MB"} {
		if got := saidSize(in); got != want {
			t.Errorf("saidSize(%d) = %q, want %q", in, got, want)
		}
	}
	for in, want := range map[string]string{"a.txt": "TXT", "a.markdown": "FILE", ".bashrc": "FILE", "noext": "FILE", "a.PnG": "PNG"} {
		if got := kindOf(in); got != want {
			t.Errorf("kindOf(%q) = %q, want %q", in, got, want)
		}
	}
}
