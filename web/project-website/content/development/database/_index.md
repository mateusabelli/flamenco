---
title: Database
weight: 50
---

Flamenco Manager and Worker use SQLite as database, sqlc as object-relational
mapper, and Goose for schema migrations.

{{< hint type=important >}}
Some of these tools assume that you have your database in
`flamenco-manager.sqlite`. Even though Flamenco Manager can store its database
anywhere, these development tools are not as flexible.

So for simplicity sake, set `database: flamenco-manager.sqlite` in your
`flamenco-manager.yaml`.
{{< /hint >}}

## SQLC

Flamenco uses [sqlc](https://sqlc.dev/) for interfacing with its SQLite database.

### Installing SQLC

SQLC can be used via `go tool sqlc`. This will run the tool, downloading &
building it if necessary. This does depend on a C/C++ compiler, so if you do not
have one, or get build errors, the [precompiled sqlc binaries][sqlc-precompiled]
work just as well. Choose whatever works for you.

Because of the above, SQLC is not part of `make with-deps` and not included in
`make generate-go`.

### Using SQLC

The Manager and Worker SQL files can be found in
`internal/manager/persistence/sqlc` and `internal/worker/persistence/sqlc`.
After updating the `.sql` files, run:

```shell
> go tool sqlc generate   # Easiest to get running, if it works for you.
> sqlc generate           # If you got the precompiled binaries.
```

For handling schema changes and database versioning, see below.

{{< hint type=note >}}
Running sqlc itself is only necessary to regenerate the database code. Once
generated, the code is independent of sqlc.
{{< /hint >}}

[sqlc-precompiled]: https://docs.sqlc.dev/en/latest/overview/install.html#downloads

### Handling Schema changes

Database schema changes are managed with [Goose][goose]. Every change is defined
in a separate SQL file, and has the queries to make the change and to roll it
back. Of course not all changes can be losslessly rolled back.

SQLC needs to know the final schema those Goose migrations produced. After
adding a migration, you can use this little helper tool to regenerate the SQLC
schema file:

```sh
make db-migrate-up
go run ./cmd/sqlc-export-schema
go tool sqlc generate
```

[goose]: https://github.com/pressly/goose
