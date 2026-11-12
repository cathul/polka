package main

import (
	"archive/zip"
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/vestigiumincaligne/polka/internal/store"
)

func TestCLIReplacePreservesDigests(t *testing.T) {
	dir := t.TempDir()
	catalog := filepath.Join(dir, "books.inpx")
	writeCatalog := func(reverse bool) {
		t.Helper()
		f, err := os.Create(catalog)
		if err != nil {
			t.Fatal(err)
		}
		z := zip.NewWriter(f)
		w, err := z.Create("books.inp")
		if err != nil {
			t.Fatal(err)
		}
		lines := []string{
			"Author,,\x04sf\x04Original\x04\x04\x04100\x041000\x04100\x040\x04fb2\r\n",
			"Author,,\x04sf\x04Other\x04\x04\x04200\x042000\x04200\x040\x04fb2\r\n",
		}
		if reverse {
			lines[0], lines[1] = lines[1], lines[0]
		}
		for _, line := range lines {
			if _, err := io.WriteString(w, line); err != nil {
				t.Fatal(err)
			}
		}
		if err := z.Close(); err != nil {
			t.Fatal(err)
		}
		if err := f.Close(); err != nil {
			t.Fatal(err)
		}
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	args := []string{"--data-dir", dir, "--inpx", catalog}
	writeCatalog(false)
	if err := runImport(log, args); err != nil {
		t.Fatal(err)
	}
	// The same explicit data-dir is used for every call, never the home directory.
	dbPath := filepath.Join(dir, "polka.db")
	st, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.SetBookDigest(context.Background(), "digest", 1); err != nil {
		t.Fatal(err)
	}
	st.Close()
	writeCatalog(true)
	if err := runImport(log, append(args, "--replace")); err != nil {
		t.Fatal(err)
	}
	st, err = store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if id, err := st.BookIDByDigest(context.Background(), "digest"); err != nil || id != 1 {
		t.Fatalf("CLI reimport digest = %d %v", id, err)
	}
}
