package server

import (
	"archive/zip"
	"bytes"
	"context"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRememberSuccessfulInpxPathOnly(t *testing.T) {
	ts, client, st := newManageServer(t)
	var catalog bytes.Buffer
	z := zip.NewWriter(&catalog)
	w, err := z.Create("books.inp")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte("Author,,\x04sf\x04Book\x04\x04\x041\x04100\x041\x040\x04fb2\r\n")); err != nil {
		t.Fatal(err)
	}
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	first := filepath.Join(dir, "first.inpx")
	second := filepath.Join(dir, "second.inpx")
	bad := filepath.Join(dir, "bad.inpx")
	for _, path := range []string{first, second} {
		if err := os.WriteFile(path, catalog.Bytes(), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(bad, []byte("not a ZIP"), 0600); err != nil {
		t.Fatal(err)
	}
	submit := func(path string, upload bool, wantPhase string) {
		t.Helper()
		var body bytes.Buffer
		mw := multipart.NewWriter(&body)
		if err := mw.WriteField("replace", "1"); err != nil {
			t.Fatal(err)
		}
		if upload {
			part, err := mw.CreateFormFile("file", "uploaded.inpx")
			if err != nil {
				t.Fatal(err)
			}
			if _, err := part.Write(catalog.Bytes()); err != nil {
				t.Fatal(err)
			}
		} else {
			if err := mw.WriteField("path", path); err != nil {
				t.Fatal(err)
			}
		}
		mw.Close()
		req, _ := http.NewRequest("POST", ts.URL+"/admin/import/inpx", &body)
		req.Header.Set("Content-Type", mw.FormDataContentType())
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Fatal(resp.StatusCode)
		}
		deadline := time.Now().Add(5 * time.Second)
		for {
			status := getJSONWith(t, client, ts.URL+"/admin/import/status")
			if status["running"] == false {
				if status["phase"] != wantPhase {
					t.Fatalf("import status: %v", status)
				}
				break
			}
			if time.Now().After(deadline) {
				t.Fatal("import timed out")
			}
			time.Sleep(5 * time.Millisecond)
		}
	}
	check := func(want string) {
		t.Helper()
		settings := getJSONWith(t, client, ts.URL+"/admin/settings")
		if settings["lastInpxPath"] != want {
			t.Fatalf("remembered path: %v want %s", settings["lastInpxPath"], want)
		}
	}
	submit(first, false, "done")
	check(first)
	submit(bad, false, "error")
	check(first)
	if n, err := st.BookCount(context.Background()); err != nil || n != 1 {
		t.Fatalf("invalid input destroyed catalog: %d %v", n, err)
	}
	submit("", true, "done")
	check(first)
	submit(second, false, "done")
	check(second)
}
