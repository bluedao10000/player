package migrations

import (
	_ "embed"

	"gorm.io/gorm"
)

//go:embed 066_workbench.sql
var workbenchSchema string

func addWorkbench(db *gorm.DB) error {
	return db.Exec(workbenchSchema).Error
}
