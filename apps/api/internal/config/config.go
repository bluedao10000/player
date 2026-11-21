// Package config reads process configuration without mutating the environment.
package config

import "github.com/spf13/viper"

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
