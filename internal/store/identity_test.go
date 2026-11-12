package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestHistoricalIDsAreNeverRecycled(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "catalog.db")
	st, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	original := BookInput{Title: "Original", Folder: "a.zip", File: "1", Ext: "fb2", Size: 100, LibID: "1"}
	id, err := st.AddBook(ctx, &original)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Clear(ctx); err != nil {
		t.Fatal(err)
	}
	other, err := st.AddBook(ctx, &BookInput{Title: "Unrelated", Folder: "a.zip", File: "2", Ext: "fb2"})
	if err != nil {
		t.Fatal(err)
	}
	if other <= id {
		t.Fatalf("recycled ID: %d <= %d", other, id)
	}
	st.Close()
	st, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if err := st.Clear(ctx); err != nil {
		t.Fatal(err)
	}
	restored, err := st.AddBook(ctx, &original)
	if err != nil {
		t.Fatal(err)
	}
	if restored != id {
		t.Fatalf("lost historical ID: %d != %d", restored, id)
	}
}

func TestDigestRematchesChangedLegacyID(t *testing.T) {
	ctx := context.Background()
	st, err := Open(filepath.Join(t.TempDir(), "catalog.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	id, err := st.AddBook(ctx, &BookInput{Title: "Legacy", Folder: "a.zip", File: "1", Ext: "fb2"})
	if err != nil {
		t.Fatal(err)
	}
	if err := st.SetBookDigest(ctx, "digest", id); err != nil {
		t.Fatal(err)
	}
	// Model a catalog whose row ID was changed externally/by an older importer.
	if _, err := st.DB().Exec(`UPDATE books SET id=99 WHERE id=?`, id); err != nil {
		t.Fatal(err)
	}
	if err := st.rematchDigests(ctx); err != nil {
		t.Fatal(err)
	}
	if got, err := st.BookIDByDigest(ctx, "digest"); err != nil || got != 99 {
		t.Fatalf("legacy rematch: %d %v", got, err)
	}
}

func TestImportRejectsCatalogWithOnlyDeletedBooks(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ctx := context.Background()
	id, err := st.AddBook(ctx, &BookInput{Title: "Hidden", Folder: "a", File: "1", Ext: "fb2"})
	if err != nil {
		t.Fatal(err)
	}
	if err := st.SetBookDeleted(ctx, id, true); err != nil {
		t.Fatal(err)
	}
	if session, err := st.NewImport(ctx); err == nil {
		session.Abort()
		t.Fatal("import accepted a nonempty hidden catalog")
	}
}

func TestImportPreparationFailureReleasesConnection(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if _, err := st.db.Exec(`DROP TABLE book_keywords`); err != nil {
		t.Fatal(err)
	}
	if session, err := st.NewImport(context.Background()); err == nil {
		session.Abort()
		t.Fatal("expected preparation failure")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := st.db.PingContext(ctx); err != nil {
		t.Fatalf("failed import left a transaction holding the only connection: %v", err)
	}
}
