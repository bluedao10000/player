package schema

import _ "embed"

//go:embed migrations/lock_migrations.sql
var LockMigrations string
