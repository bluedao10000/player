package workbench

import _ "embed"

//go:embed usage/backup_status.sql
var BackupStatus string

//go:embed usage/episode_queue.sql
var EpisodeQueue string

//go:embed usage/reader_releases.sql
var ReaderReleases string

//go:embed usage/episode_inventory.sql
var EpisodeInventory string

//go:embed usage/model_usage.sql
var ModelUsage string

//go:embed usage/calls.sql
var Calls string
