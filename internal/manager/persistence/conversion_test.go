package persistence

import (
	"database/sql"
	"reflect"
	"testing"
)

func Test_nullTimeToUTC(t *testing.T) {

	inUTC := mustParseTime("2024-11-11T20:12:47Z")
	inBangkok := mustParseTime("2024-11-12T03:12:47+07:00")

	tests := []struct {
		name string
		arg  sql.NullTime
		want sql.NullTime
	}{
		{"zero", sql.NullTime{}, sql.NullTime{}},
		{"invalid-nonzero-utc",
			sql.NullTime{Time: inUTC, Valid: false},
			sql.NullTime{Time: inUTC, Valid: false},
		},
		{"valid-nonzero-utc",
			sql.NullTime{Time: inUTC, Valid: true},
			sql.NullTime{Time: inUTC, Valid: true},
		},
		{"invalid-nonzero-bangkok",
			sql.NullTime{Time: inBangkok, Valid: false},
			sql.NullTime{Time: inUTC, Valid: false},
		},
		{"valid-nonzero-bangkok",
			sql.NullTime{Time: inBangkok, Valid: true},
			sql.NullTime{Time: inUTC, Valid: true},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := nullTimeToUTC(tt.arg); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("nullTimeToUTC() = %v, want %v", got, tt.want)
			}
		})
	}
}
