package migrations

import (
	"time"

	"gorm.io/gorm"
)

// These migration-local types freeze the schema at version 049. The backup run
// already exists from 047; only its key is needed to describe the foreign key.
type backupRunKey struct {
	ID string `gorm:"column:job_uid;type:uuid;primaryKey"`
}

func (backupRunKey) TableName() string { return "backup_runs" }

type backupFailedAttempt struct {
	PodUID     string       `gorm:"column:pod_uid;type:uuid;primaryKey"`
	JobUID     string       `gorm:"column:job_uid;type:uuid;not null;index:backup_attempts_by_job,priority:1"`
	FinishedAt time.Time    `gorm:"type:timestamptz;not null;index:backup_attempts_by_job,priority:2"`
	Reason     string       `gorm:"type:text;not null"`
	Run        backupRunKey `gorm:"foreignKey:JobUID;references:ID;constraint:backup_failed_attempts_job_uid_fkey,"`
}

func (backupFailedAttempt) TableName() string { return "backup_failed_attempts" }

func addBackupAttempts(tx *gorm.DB) error {
	return tx.Migrator().CreateTable(&backupFailedAttempt{})
}
