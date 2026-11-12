package server

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestKosyncBridgeAfterWebReplace(t *testing.T) {
	f := newFullServer(t, "required")
	base := f.ts.URL
	postJSON(t, f.reader, base+"/api/v1/me/kosync", map[string]string{"password": "device-pass-123"}).Body.Close()
	resp := do(t, f.reader, "GET", base+"/Images/fb2/"+itoa64(f.bookID), "")
	data, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	doc := referenceDigest(data)
	// Reverse book order so the old book ID now identifies a different file.
	var catalog bytes.Buffer
	zw := zip.NewWriter(&catalog)
	entry, err := zw.Create("arch-001.inp")
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range []string{"101", "100"} {
		fields := []string{"Толстой,Лев,", "prose_classic", "Book " + file, "", "", file, fmt.Sprint(len(testFB2)), "", "0", "fb2", "2024-01-01", "ru"}
		if _, err := io.WriteString(entry, strings.Join(fields, "\x04")+"\r\n"); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	if err := mw.WriteField("replace", "1"); err != nil {
		t.Fatal(err)
	}
	part, err := mw.CreateFormFile("file", "catalog.inpx")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(catalog.Bytes()); err != nil {
		t.Fatal(err)
	}
	if err := mw.Close(); err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest("POST", base+"/admin/import/inpx", &body)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	resp, err = f.admin.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("replace: %d", resp.StatusCode)
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		status := getJSONWith(t, f.admin, base+"/admin/import/status")
		if status["running"] == false {
			if status["phase"] != "done" {
				t.Fatalf("import: %v", status)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("import timed out")
		}
		time.Sleep(5 * time.Millisecond)
	}
	id, err := f.st.BookIDByDigest(context.Background(), doc)
	if err != nil || id != f.bookID {
		t.Fatalf("remapped digest: %d %v", id, err)
	}
	code, _ := kosyncDo(t, base, "PUT", "/syncs/progress", "reader", md5hex("device-pass-123"),
		`{"document":"`+doc+`","progress":"/body/p[1]","percentage":0.73,"device":"Kindle"}`)
	if code != 200 {
		t.Fatalf("kosync: %d", code)
	}
	progress := getJSONWith(t, f.reader, base+"/api/v1/read/"+itoa64(id)+"/progress")
	if progress["progress"] != 0.73 {
		t.Fatalf("new book progress: %v", progress)
	}
	other := getJSONWith(t, f.reader, base+"/api/v1/read/"+itoa64(f.bookID2)+"/progress")
	if other["progress"] == 0.73 {
		t.Fatalf("progress attached to reused ID: %v", other)
	}
}
