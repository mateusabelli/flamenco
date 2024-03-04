# SQL Migrations

The files here are run by [Goose][goose], the database migration tool.

[goose]: https://pkg.go.dev/github.com/pressly/goose/v3

These are embedded into the Flamenco Manager executable, and automatically run
on startup.

## Foreign Key Constraints

Foreign Key constraints (FKCs) are optional in SQLite, and always enabled by
Flamenco Manager. These are temporarily disabled during database migration
itself. This means you can replace a table like this, without `ON DELETE`
effects running.

```sql
INSERT INTO `temp_table` SELECT field1, field2, etc FROM `actual_table`;
DROP TABLE `actual_table`;
ALTER TABLE `temp_table` RENAME TO `actual_table`;
```

Note that the `SELECT` clause lists each field specifically. This is to ensure
that they are selected in the expected order. Without this, data can get
mangled.
