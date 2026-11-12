package store

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/vestigiumincaligne/polka/internal/sqlitedrv"
)

func TestDigestMigrationV4(t *testing.T) {
	path := filepath.Join(t.TempDir(), "v4.db")
	db, err := sqlitedrv.Open(path, sqlitedrv.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(schemaV1 + `
 INSERT INTO folders(id,name) VALUES(1,'a.zip');
 INSERT INTO books(id,folder_id,title,file,ext,size,lib_id) VALUES(9,1,'Known','9','fb2',99,'900');
 INSERT INTO book_digests(digest,book_id) VALUES('known',9),('orphan',999);
 PRAGMA user_version=4;`); err != nil {
		t.Fatal(err)
	}
	db.Close()
	for i := 0; i < 2; i++ {
		st, err := Open(path)
		if err != nil {
			t.Fatal(err)
		}
		ctx := context.Background()
		if id, err := st.BookIDByDigest(ctx, "known"); err != nil || id != 9 {
			t.Fatalf("migrated = %d, %v", id, err)
		}
		if _, err := st.BookIDByDigest(ctx, "orphan"); !errors.Is(err, ErrNotFound) {
			t.Fatalf("orphan: %v", err)
		}
		var n int
		if err := st.DB().QueryRow(`SELECT count(*) FROM book_digests`).Scan(&n); err != nil || n != 2 {
			t.Fatalf("retained digests: %d %v", n, err)
		}
		st.Close()
	}
}

func TestDigestReimportSafety(t *testing.T) {
	for _, scenario := range []string{"missing", "size", "libid", "folder", "file", "ext", "duplicate", "deleted", "unchanged", "no-libid"} {
		t.Run(scenario, func(t *testing.T) {
			ctx := context.Background()
			st, err := Open(filepath.Join(t.TempDir(), "test.db"))
			if err != nil {
				t.Fatal(err)
			}
			defer st.Close()
			original := BookInput{LibID: "100", Title: "Book", Folder: "a.zip", File: "100", Ext: "fb2", Size: 123}
			if scenario == "no-libid" {
				original.LibID = ""
			}
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
			replacement := original
			switch scenario {
			case "size":
				replacement.Size++
			case "libid":
				replacement.LibID = "other"
			case "folder":
				replacement.Folder = "other.zip"
			case "file":
				replacement.File = "other"
			case "ext":
				replacement.Ext = "epub"
			case "deleted":
				replacement.Deleted = true
			}
			im, err := st.NewImport(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer im.Abort()
			if scenario != "missing" {
				if err := im.Add(&replacement); err != nil {
					t.Fatal(err)
				}
			}
			if scenario == "duplicate" {
				if err := im.Add(&replacement); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := im.Finish(); err != nil {
				t.Fatal(err)
			}
			got, err := st.BookIDByDigest(ctx, "digest")
			if scenario == "unchanged" || scenario == "no-libid" {
				if err != nil || got != 1 {
					t.Fatalf("exact match = %d %v", got, err)
				}
			} else if !errors.Is(err, ErrNotFound) {
				t.Fatalf("unsafe mapping = %d %v", got, err)
			}
			// A later complete import can recover an unresolved digest.
			if err := st.Clear(ctx); err != nil {
				t.Fatal(err)
			}
			im, err = st.NewImport(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer im.Abort()
			if err := im.Add(&original); err != nil {
				t.Fatal(err)
			}
			if _, err := im.Finish(); err != nil {
				t.Fatal(err)
			}
			if _, err := st.BookIDByDigest(ctx, "digest"); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestDigestInterruptedImportAndLateDownload(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "test.db")
	st, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	old := BookInput{Title: "Old", Folder: "a.zip", File: "old", Ext: "fb2", Size: 123}
	id, err := st.AddBook(ctx, &old)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := st.BookFile(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.SetBookDigest(ctx, "digest", id); err != nil {
		t.Fatal(err)
	}
	if err := st.Clear(ctx); err != nil {
		t.Fatal(err)
	}
	im, err := st.NewImport(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := im.Add(&old); err != nil {
		t.Fatal(err)
	}
	// Simulate a committed early batch followed by a crash, without a scale test.
	if err := im.commitBatch(); err != nil {
		t.Fatal(err)
	}
	im.Abort()
	if err := st.SetBookDigestForFile(ctx, "late", snapshot); err != nil {
		t.Fatal(err)
	}
	st.Close()
	st, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	for _, digest := range []string{"digest", "late"} {
		if id, err := st.BookIDByDigest(ctx, digest); !errors.Is(err, ErrNotFound) {
			t.Fatalf("partial catalog mapped %s to %d: %v", digest, id, err)
		}
	}
	if err := st.Clear(ctx); err != nil {
		t.Fatal(err)
	}
	im, err = st.NewImport(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer im.Abort()
	if err := im.Add(&BookInput{Title: "New", Folder: "a.zip", File: "new", Ext: "fb2"}); err != nil {
		t.Fatal(err)
	}
	if err := im.Add(&old); err != nil {
		t.Fatal(err)
	}
	if _, err := im.Finish(); err != nil {
		t.Fatal(err)
	}
	// A second delayed download completes after the import, with the original ID.
	if err := st.SetBookDigestForFile(ctx, "late-after-finish", snapshot); err != nil {
		t.Fatal(err)
	}
	for _, digest := range []string{"digest", "late", "late-after-finish"} {
		if id, err := st.BookIDByDigest(ctx, digest); err != nil || id != snapshot.ID {
			t.Fatalf("recovered %s = %d %v", digest, id, err)
		}
	}
}

func TestClearRollsBackDigestDetachment(t *testing.T) {
	ctx := context.Background()
	st, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	id, err := st.AddBook(ctx, &BookInput{Title: "Book", Folder: "a.zip", File: "1", Ext: "fb2"})
	if err != nil {
		t.Fatal(err)
	}
	if err := st.SetBookDigest(ctx, "digest", id); err != nil {
		t.Fatal(err)
	}
	if _, err := st.DB().Exec(`CREATE TRIGGER prevent_clear BEFORE DELETE ON books BEGIN SELECT RAISE(ABORT,'test failure'); END`); err != nil {
		t.Fatal(err)
	}
	if err := st.Clear(ctx); err == nil {
		t.Fatal("expected clear failure")
	}
	if got, err := st.BookIDByDigest(ctx, "digest"); err != nil || got != id {
		t.Fatalf("rollback mapping = %d %v", got, err)
	}
}
