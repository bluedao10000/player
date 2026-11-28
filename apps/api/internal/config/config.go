// Package config reads process configuration without mutating the environment.
package config

import (
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

// ArchiveDir returns the configured or per-user archive root. If the user
// directory is unavailable, the path is relative and only valid for the legacy
// model-usage outbox. Snapshot callers must propagate the error.
func ArchiveDir() (string, error) {
	if dir := os.Getenv("KNOWLEDGE_ARCHIVE_DIR"); dir != "" {
		return dir, nil
	}
	base, err := os.UserConfigDir()
	return filepath.Join(base, "tidepost", "knowledge-archive"), err
}

type API struct{ Host, Port string }
type Sync struct {
	AgentURL     string
	MediaEnabled bool
}

func environment() *viper.Viper {
	v := viper.New()
	v.AutomaticEnv()
	return v
}
func LoadAPI() API {
	v := environment()
	v.SetDefault("API_HOST", "127.0.0.1")
	v.SetDefault("API_PORT", "4311")
	return API{Host: v.GetString("API_HOST"), Port: v.GetString("API_PORT")}
}
func LoadSync() Sync {
	v := environment()
	return Sync{AgentURL: v.GetString("KNOWLEDGE_RESEARCH_URL"), MediaEnabled: v.GetString("MEDIA_WORKER_ENABLED") == "true"}
}
func Database() (url string, verifyOnly bool) {
	v := environment()
	return v.GetString("DATABASE_URL"), v.GetString("KNOWLEDGE_VERIFY_SCHEMA_ONLY") == "true"
}
