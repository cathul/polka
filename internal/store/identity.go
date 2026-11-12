package store

import (
	"context"
	"database/sql"
	"errors"
)

// This ledger is independent of the replaceable catalog. Missing books retain
// their reserved IDs, so user data cannot attach to unrelated new arrivals.
const identitySchema = `
CREATE TABLE IF NOT EXISTS catalog_identities (
 book_id INTEGER PRIMARY KEY,
 folder TEXT NOT NULL, file TEXT NOT NULL, ext TEXT NOT NULL,
 size INTEGER NOT NULL, lib_id TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_catalog_identity ON catalog_identities(folder,file,ext,size,lib_id);
INSERT OR IGNORE INTO catalog_identities(book_id,folder,file,ext,size,lib_id)
 SELECT b.id,f.name,b.file,b.ext,b.size,b.lib_id FROM books b JOIN folders f ON f.id=b.folder_id;
`

// reserveBookID reuses only one exact historical identity, and only once per
// catalog. Duplicate identities get fresh IDs; digest rematching rejects them.
func reserveBookID(ctx context.Context, tx *sql.Tx, b *BookInput) (int64, error) {
	args := []any{b.Folder, b.File, b.Ext, b.Size, b.LibID}
	var id int64
	err := tx.QueryRowContext(ctx, `SELECT candidate.book_id FROM (
  SELECT min(book_id) AS book_id FROM catalog_identities
  WHERE folder=? AND file=? AND ext=? AND size=? AND lib_id=? HAVING count(*)=1
 ) AS candidate WHERE NOT EXISTS(SELECT 1 FROM books WHERE id=candidate.book_id)`, args...).Scan(&id)
	if err == nil {
		return id, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return 0, err
	}
	// Allocate inside this transaction rather than caching a high-water mark:
	// uploads between import batches must not collide with the next imported ID.
	err = tx.QueryRowContext(ctx, `INSERT INTO catalog_identities(book_id,folder,file,ext,size,lib_id)
 SELECT (SELECT max(n)+1 FROM (
  SELECT coalesce(max(book_id),0) AS n FROM catalog_identities
  UNION ALL SELECT coalesce(max(id),0) FROM books)),?,?,?,?,?
 RETURNING book_id`, args...).Scan(&id)
	return id, err
}
