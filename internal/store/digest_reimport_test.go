package store

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
)

func TestDigestSurvivesReorderedImportWithStableIDs(t *testing.T) {
	ctx := context.Background()
	st, err := Open(filepath.Join(t.TempDir(), "catalog.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	original := BookInput{LibID: "100", Title: "Original", Folder: "a.zip", File: "100", Ext: "fb2", Size: 1000}
	id, err := st.AddBook(ctx, &original)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.SetBookDigest(ctx, "digest", id); err != nil {
		t.Fatal(err)
	}
	if err := st.Clear(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := st.BookIDByDigest(ctx, "digest"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("after clear: %v", err)
	}
	im, err := st.NewImport(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer im.Abort()
	if err := im.Add(&BookInput{Title: "Different", Folder: "a.zip", File: "200", Ext: "fb2"}); err != nil {
		t.Fatal(err)
	}
	if err := im.Add(&original); err != nil {
		t.Fatal(err)
	}
	if _, err := im.Finish(); err != nil {
		t.Fatal(err)
	}
	got, err := st.BookIDByDigest(ctx, "digest")
	if err != nil || got != id {
		t.Fatalf("digest = %d, %v; want stable ID (old ID %d)", got, err, id)
	}
}
