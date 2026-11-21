// Package migrations applies ordered PostgreSQL changes using the existing deployment ledger.
package migrations

import (
	"embed"
	"fmt"
	"time"

	"gorm.io/gorm"

	queries "anime-agent/database/queries/schema"
)

//go:embed legacy/*.sql
var history embed.FS

type appliedVersion struct {
	Version   string    `gorm:"type:text;primaryKey"`
	AppliedAt time.Time `gorm:"type:timestamptz;not null;default:now()"`
}

func (appliedVersion) TableName() string { return "knowledge_migrations" }

type migration struct {
	version string
	up      func(*gorm.DB) error
}

func versions() []migration {
	var steps []migration
	for i := 1; i <= 48; i++ {
		version := fmt.Sprintf("%03d.sql", i)
		steps = append(steps, migration{version, func(tx *gorm.DB) error {
			sql, err := history.ReadFile("legacy/" + version)
			if err != nil {
				return err
			}
			return tx.Exec(string(sql)).Error
		}})
	}
	return append(steps, migration{"049.sql", addBackupAttempts})
}

// Apply and the previous pgx migrator share a lock and immutable version IDs.
func Apply(db *gorm.DB) error {
	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec(queries.LockMigrations).Error; err != nil {
			return err
		}
		if !tx.Migrator().HasTable(&appliedVersion{}) {
			if err := tx.Migrator().CreateTable(&appliedVersion{}); err != nil {
				return err
			}
		}
		var records []appliedVersion
		if err := tx.Find(&records).Error; err != nil {
			return err
		}
		installed := make(map[string]bool, len(records))
		for _, record := range records {
			installed[record.Version] = true
		}
		for _, step := range versions() {
			if installed[step.version] {
				continue
			}
			if err := step.up(tx); err != nil {
				return fmt.Errorf("migration %s: %w", step.version, err)
			}
			if err := tx.Create(&appliedVersion{Version: step.version}).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// Verify uses SELECT only so readers and restricted workers do not need DDL rights.
func Verify(db *gorm.DB) error {
	var expected []string
	for _, step := range versions() {
		expected = append(expected, step.version)
	}
	var installed int64
	if err := db.Model(&appliedVersion{}).Where(map[string]any{"version": expected}).Count(&installed).Error; err != nil {
		return err
	}
	if installed != int64(len(expected)) {
		return fmt.Errorf("schema requires %d migrations, found %d", len(expected), installed)
	}
	return nil
}
