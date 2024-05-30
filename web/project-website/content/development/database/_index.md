---
title: Database
weight: 50
---

Flamenco Manager and Worker use SQLite as database, and GORM as
object-relational mapper (but see the note below).

Since SQLite has limited support for altering table schemas, migration requires
copying old data to a temporary table with the new schema, then swap out the
tables.

## SQLC

Flamenco mostly uses [GORM](https://gorm.io/) for interfacing with its SQLite database. This
is gradually being phased out, to be replaced with [SQLC](https://sqlc.dev/).

### Installing & using SQLC

SQLC can be installed ([installation docs][sqlc-install]) with a `go install`
command just like any other Go package, but that does depend on a C/C++
compiler:

```sh
go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest
```

The [precompiled sqlc binaries][sqlc-precompiled] work just as well, so choose
whatever works for you.

{{< hint type=important >}}
Installing sqlc itself is only necessary to regenerate the database code. Once
generated, the code is independent of sqlc.

Since installing sqlc via `go install` requires a C/C++ compiler, it is **not** part
of the `make with-deps` script. Because of this, it is also **not** included in the
`make generate-go` script.
{{< /hint >}}

[sqlc-install]: https://docs.sqlc.dev/en/latest/overview/install.html
[sqlc-precompiled]: https://docs.sqlc.dev/en/latest/overview/install.html#downloads

### Handling Schema changes

Database schema changes are managed with [Goose][goose]. Every change is defined
in a separate SQL file, and has the queries to make the change and to roll it
back. Of course the roll-back is only possible when no data was removed.

SQLC needs to know the final schema those Goose migrations produced. To generate
the SQLC schema from the database itself, run:
```sh
make db-migrate-up
go run ./cmd/sqlc-export-schema
```

To generate Go code with SQLC after changing `schema.sql` or `queries.sql`:
```sh
go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest
sqlc generate
```

[goose]: https://github.com/pressly/goose
