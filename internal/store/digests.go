package store

import (
	"context"
	"database/sql"
	"errors"
)

// SetBookDigest is for a file belonging to the current catalog. Streamed
// downloads must use SetBookDigestForFile with their original file snapshot.
func (s *Store) SetBookDigest(ctx context.Context, digest string, bookID int64) error {
	f, err := s.BookFile(ctx, bookID)
	if err != nil {
		return err
	}
	return s.SetBookDigestForFile(ctx, digest, f)
}

// SetBookDigestForFile keeps the identity captured before streaming. An import
// may have replaced the row with the same ID while the download was in flight.
func (s *Store) SetBookDigestForFile(ctx context.Context, digest string, f *BookFile) error {
	_, err := s.db.ExecContext(ctx, `
 INSERT INTO book_digests(digest,book_id,folder,file,ext,size,lib_id)
 VALUES(?, (SELECT min(b.id) FROM books b JOIN folders fo ON fo.id=b.folder_id
  WHERE fo.name=? AND b.file=? AND b.ext=? AND b.size=? AND b.lib_id=?
  AND b.deleted=0 AND NOT EXISTS(SELECT 1 FROM meta WHERE key='digest_import_pending' AND value='1')
  HAVING count(*)=1),
 ?,?,?,?,?)
 ON CONFLICT(digest) DO UPDATE SET book_id=excluded.book_id, folder=excluded.folder,
 file=excluded.file, ext=excluded.ext, size=excluded.size, lib_id=excluded.lib_id`,
		digest, f.Folder, f.File, f.Ext, f.Size, f.LibID,
		f.Folder, f.File, f.Ext, f.Size, f.LibID)
	return err
}

// BookIDByDigest never trusts an old row ID by itself. Check identity again so
// deleted/mutated catalog rows cannot silently receive another file's progress.
func (s *Store) BookIDByDigest(ctx context.Context, digest string) (int64, error) {
	var id int64
	err := s.db.QueryRowContext(ctx, `SELECT b.id FROM book_digests d
 JOIN books b ON b.id=d.book_id JOIN folders f ON f.id=b.folder_id
 WHERE d.digest=? AND b.deleted=0 AND f.name=d.folder AND b.file=d.file
 AND b.ext=d.ext AND b.size=d.size AND b.lib_id=d.lib_id
 AND NOT EXISTS(SELECT 1 FROM meta WHERE key='digest_import_pending' AND value='1')`, digest).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrNotFound
	}
	return id, err
}

// rematchDigests only runs after a complete import and index rebuild. Never
// guess between editions or bind an ambiguous identity to an arbitrary row.
func (s *Store) rematchDigests(ctx context.Context) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `UPDATE book_digests AS d SET book_id=(
 SELECT min(b.id) FROM books b JOIN folders f ON f.id=b.folder_id
 WHERE f.name=d.folder AND b.file=d.file AND b.ext=d.ext AND b.size=d.size
 AND b.lib_id=d.lib_id AND b.deleted=0
 HAVING count(*)=1)`); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM meta WHERE key='digest_import_pending'`); err != nil {
		return err
	}
	return tx.Commit()
}
