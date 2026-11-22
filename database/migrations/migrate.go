// Package migrations applies ordered PostgreSQL changes using the existing deployment ledger.
package migrations

import (
	_ "embed"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
)

//go:embed init.sql
var baseline string

func section(name string) string {
	_, body, found := strings.Cut(baseline, "\n-- name: "+name+"\n")
	if !found {
		panic("missing migration section: " + name)
	}
	sql, _, _ := strings.Cut(body, "\n-- name: ")
	return sql
}

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
			return tx.Exec(section(version)).Error
		}})
	}
	return append(steps, migration{"049.sql", addBackupAttempts}, migration{"050.sql", func(tx *gorm.DB) error {
		return tx.Exec(section("050.sql")).Error
	}}, migration{"051.sql", func(tx *gorm.DB) error { return tx.Exec(section("051.sql")).Error }}, migration{"052.sql", func(tx *gorm.DB) error { return tx.Exec(section("052.sql")).Error }}, migration{"053.sql", func(tx *gorm.DB) error { return tx.Exec(section("053.sql")).Error }}, migration{"054.sql", func(tx *gorm.DB) error { return tx.Exec(section("054.sql")).Error }}, migration{"055.sql", func(tx *gorm.DB) error { return tx.Exec(section("055.sql")).Error }}, migration{"056.sql", func(tx *gorm.DB) error { return tx.Exec(section("056.sql")).Error }})
}

// Apply and the previous pgx migrator share a lock and immutable version IDs.
func Apply(db *gorm.DB) error {
	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec(section("lock")).Error; err != nil {
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
