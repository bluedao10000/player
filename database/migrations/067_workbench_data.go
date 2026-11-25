package migrations

import (
	_ "embed"

	"gorm.io/gorm"
)

//go:embed 067_workbench_data.sql
var workbenchDataSchema string

func addWorkbenchData(db *gorm.DB) error {
	return db.Exec(workbenchDataSchema).Error
}
