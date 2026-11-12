package converter

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestConversionFailureAndTimeout(t *testing.T) {
	for _, test := range []struct {
		name, script string
		timeout      time.Duration
		want         error
	}{
		{"failure", "#!/bin/sh\nexit 2\n", time.Second, ErrFailed},
		{"missing-output", "#!/bin/sh\nexit 0\n", time.Second, ErrFailed},
		{"timeout", "#!/bin/sh\nexec sleep 30\n", 20 * time.Millisecond, context.DeadlineExceeded},
	} {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			binary := filepath.Join(dir, "fbc")
			if err := os.WriteFile(binary, []byte(test.script), 0700); err != nil {
				t.Fatal(err)
			}
			if _, err := EPUB3(context.Background(), binary, filepath.Join(dir, "source.fb2"), test.timeout); !errors.Is(err, test.want) {
				t.Fatalf("got %v, want %v", err, test.want)
			}
		})
	}
}

// The full converter is built separately, never installed automatically by tests.
func TestRealEPUB3(t *testing.T) {
	binary := os.Getenv("POLKA_TEST_FBC")
	if binary == "" {
		t.Skip("set POLKA_TEST_FBC to the verified converter executable")
	}
	dir := t.TempDir()
	source := filepath.Join(dir, "source.fb2")
	book := `<?xml version="1.0" encoding="UTF-8"?><FictionBook xmlns="http://www.gribuser.ru/xml/fictionbook/2.0"><description><title-info><genre>prose_classic</genre><author><first-name>Test</first-name><last-name>Author</last-name></author><book-title>Polka conversion test</book-title><lang>en</lang></title-info><document-info><author><nickname>Polka</nickname></author><date>2026-11-11</date><id>polka-test</id><version>1.0</version></document-info></description><body><section><title><p>Chapter</p></title><p>Readable test paragraph.</p></section></body></FictionBook>`
	if err := os.WriteFile(source, []byte(book), 0600); err != nil {
		t.Fatal(err)
	}
	output, err := EPUB3(context.Background(), binary, source, 30*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	z, err := zip.OpenReader(output)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	var epub3, nav, body bool
	for _, f := range z.File {
		r, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(r)
		r.Close()
		if err != nil {
			t.Fatal(err)
		}
		if strings.HasSuffix(f.Name, ".opf") && bytes.Contains(data, []byte(`version="3.0"`)) {
			epub3 = true
		}
		if strings.Contains(f.Name, "nav") && bytes.Contains(data, []byte("Chapter")) {
			nav = true
		}
		if bytes.Contains(data, []byte("Readable test paragraph.")) {
			body = true
		}
	}
	if !epub3 || !nav || !body {
		t.Fatalf("invalid EPUB3 contents: package=%v navigation=%v text=%v", epub3, nav, body)
	}
}
