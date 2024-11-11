package persistence

// SPDX-License-Identifier: GPL-3.0-or-later

import "database/sql"

func nullTimeToUTC(t sql.NullTime) sql.NullTime {
	return sql.NullTime{
		Time:  t.Time.UTC(),
		Valid: t.Valid,
	}
}

func ptr[T any](value T) *T {
	return &value
}
