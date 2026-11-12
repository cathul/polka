package store

import (
	"database/sql"
	"errors"
	"fmt"
)

// Fork migrations have their own version keys. They share the upstream
// migration transaction but never consume upstream PRAGMA user_version values.
func migrateCustom(tx *sql.Tx) error {
	migrations := []struct{ key, schema string }{
		{"custom_digest_identity_version", schemaDigestIdentityV1},
		{"custom_stable_ids_version", identitySchema},
	}
	for _, migration := range migrations {
		var version string
		err := tx.QueryRow(`SELECT value FROM meta WHERE key=?`, migration.key).Scan(&version)
		if errors.Is(err, sql.ErrNoRows) {
			if _, err := tx.Exec(migration.schema); err != nil {
				return fmt.Errorf("%s: %w", migration.key, err)
			}
			if _, err := tx.Exec(`INSERT INTO meta(key,value) VALUES(?,'1')`, migration.key); err != nil {
				return err
			}
		} else if err != nil {
			return err
		} else if version != "1" {
			return fmt.Errorf("unsupported %s version %q", migration.key, version)
		}
	}
	return nil
}
