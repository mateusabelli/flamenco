---
title: Database
weight: 50
---

Flamenco Manager and Worker use SQLite as database, and GORM as
object-relational mapper (but see the note below).

Since SQLite has limited support for altering table schemas, migration requires
copying old data to a temporary table with the new schema, then swap out the
tables. Because of this, avoid `NOT NULL` columns, as they will be problematic
in this process.

## SQLC

Flamenco mostly uses [GORM](https://gorm.io/) for interfacing with its SQLite database. This
is gradually being phased out, to be replaced with [SQLC](https://sqlc.dev/).

To generate the SQLC schema file:
```sh
make db-migrate-up
go run ./cmd/sqlc-export-schema
```

To generate Go code with SQLC after changing `schema.sql` or `queries.sql`:
```sh
go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest
sqlc generate
```
