// Code MANUALLY written to extend the SQLC interface with some extra functions.
//
// This is to work around https://github.com/sqlc-dev/sqlc/issues/3237

package sqlc

import (
	"context"
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
	defer rows.Close()
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
