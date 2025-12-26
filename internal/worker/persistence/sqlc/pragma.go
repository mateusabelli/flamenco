// Code MANUALLY written to extend the SQLC interface with some extra functions.
//
// This is to work around https://github.com/sqlc-dev/sqlc/issues/3237

package sqlc

import (
	"context"
	"fmt"
	"time"
)

const pragmaIntegrityCheck = `PRAGMA integrity_check`

type PragmaIntegrityCheckResult struct {
	Description string
}

func (q *Queries) PragmaIntegrityCheck(ctx context.Context) ([]PragmaIntegrityCheckResult, error) {
	rows, err := q.db.QueryContext(ctx, pragmaIntegrityCheck)
	if err != nil {
		return nil, err
	}
	var items []PragmaIntegrityCheckResult
	for rows.Next() {
		var i PragmaIntegrityCheckResult
		if err := rows.Scan(
			&i.Description,
		); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

// SQLite doesn't seem to like SQL parameters for `PRAGMA`, so `PRAGMA foreign_keys = ?` doesn't work.
const pragmaForeignKeysEnable = `PRAGMA foreign_keys = 1`
const pragmaForeignKeysDisable = `PRAGMA foreign_keys = 0`

func (q *Queries) PragmaForeignKeysSet(ctx context.Context, enable bool) error {
	var sql string
	if enable {
		sql = pragmaForeignKeysEnable
	} else {
		sql = pragmaForeignKeysDisable
	}

	_, err := q.db.ExecContext(ctx, sql)
	return err
}

const pragmaForeignKeys = `PRAGMA foreign_keys`

func (q *Queries) PragmaForeignKeysGet(ctx context.Context) (bool, error) {
	row := q.db.QueryRowContext(ctx, pragmaForeignKeys)
	var fkEnabled bool
	err := row.Scan(&fkEnabled)
	return fkEnabled, err
}

const pragmaForeignKeyCheck = `PRAGMA foreign_key_check`

type PragmaForeignKeyCheckResult struct {
	Table  string
	RowID  int
	Parent string
	FKID   int
}

func (q *Queries) PragmaForeignKeyCheck(ctx context.Context) ([]PragmaForeignKeyCheckResult, error) {
	rows, err := q.db.QueryContext(ctx, pragmaForeignKeyCheck)
	if err != nil {
		return nil, err
	}
	var items []PragmaForeignKeyCheckResult
	for rows.Next() {
		var i PragmaForeignKeyCheckResult
		if err := rows.Scan(
			&i.Table,
			&i.RowID,
			&i.Parent,
			&i.FKID,
		); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (q *Queries) PragmaBusyTimeout(ctx context.Context, busyTimeout time.Duration) error {
	sql := fmt.Sprintf("PRAGMA busy_timeout = %d", busyTimeout.Milliseconds())
	_, err := q.db.ExecContext(ctx, sql)
	return err
}

const pragmaJournalModeWAL = `PRAGMA journal_mode = WAL`

func (q *Queries) PragmaJournalModeWAL(ctx context.Context) error {
	_, err := q.db.ExecContext(ctx, pragmaJournalModeWAL)
	return err
}

const pragmaSynchronousNormal = `PRAGMA synchronous = normal`

func (q *Queries) PragmaSynchronousNormal(ctx context.Context) error {
	_, err := q.db.ExecContext(ctx, pragmaSynchronousNormal)
	return err
}

const vacuum = `VACUUM`

func (q *Queries) Vacuum(ctx context.Context) error {
	_, err := q.db.ExecContext(ctx, vacuum)
	return err
}
