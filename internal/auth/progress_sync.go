package auth

import (
	"context"
	"database/sql"
)

// Use a strictly newer timestamp for an explicit read/delete, even in the same
// millisecond or when an offline client's clock was ahead. A deletion is not a
// ban: any new local write (including KOReader) removes its marker.
func progressTimestamp(ctx context.Context, tx *sql.Tx, userID, bookID int64) (string, error) {
	var timestamp string
	err := tx.QueryRowContext(ctx, `SELECT max(strftime('%Y-%m-%d %H:%M:%f','now'),
 coalesce(strftime('%Y-%m-%d %H:%M:%f',max(updated_at),'+0.001 seconds'),''))
 FROM (SELECT updated_at FROM reading_progress WHERE user_id=? AND book_id=?
 UNION ALL SELECT updated_at FROM reading_progress_deleted WHERE user_id=? AND book_id=?)`, userID, bookID, userID, bookID).Scan(&timestamp)
	return timestamp, err
}

// Keep a deletion in state-based sync so an old snapshot cannot resurrect it.
// The optional JSON flag is understood by develop desktop/server versions;
// older clients' ordinary snapshots cannot overwrite an equal/newer deletion.
func (s *Service) mergeProgress(ctx context.Context, userID int64, p ProgressState) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var latest sql.NullString
	if err := tx.QueryRowContext(ctx, `SELECT max(updated_at) FROM (
 SELECT updated_at FROM reading_progress WHERE user_id=? AND book_id=?
 UNION ALL SELECT updated_at FROM reading_progress_deleted WHERE user_id=? AND book_id=?)`, userID, p.BookID, userID, p.BookID).Scan(&latest); err != nil {
		return err
	}
	if latest.Valid && (latest.String > p.UpdatedAt || latest.String == p.UpdatedAt && !p.Deleted) {
		return nil
	}
	if p.Deleted {
		if _, err := tx.ExecContext(ctx, `DELETE FROM reading_progress WHERE user_id=? AND book_id=?`, userID, p.BookID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO reading_progress_deleted(user_id,book_id,updated_at) VALUES(?,?,?)
  ON CONFLICT(user_id,book_id) DO UPDATE SET updated_at=excluded.updated_at`, userID, p.BookID, p.UpdatedAt); err != nil {
			return err
		}
	} else {
		if _, err := tx.ExecContext(ctx, `INSERT INTO reading_progress(user_id,book_id,chapter,position,overall,locator,updated_at)
  VALUES(?,?,?,?,?,?,?) ON CONFLICT(user_id,book_id) DO UPDATE SET chapter=excluded.chapter,
  position=excluded.position,overall=excluded.overall,locator=excluded.locator,updated_at=excluded.updated_at`, userID, p.BookID, p.Chapter, p.Position, p.Overall, p.Locator, p.UpdatedAt); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM reading_progress_deleted WHERE user_id=? AND book_id=?`, userID, p.BookID); err != nil {
			return err
		}
	}
	return tx.Commit()
}
