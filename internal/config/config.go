package config

import "os"

type Config struct {
	ListenAddr         string
	AgentPublicAddr    string
	JWTSecret          string
	DBPath             string
	BootstrapAdminUser string
	BootstrapAdminPass string
}

func FromEnv() Config {
	cfg := Config{
		ListenAddr:         getOrDefault("GMC_LISTEN_ADDR", ":8080"),
		AgentPublicAddr:    getOrDefault("GMC_AGENT_PUBLIC_ADDR", ""),
		JWTSecret:          getOrDefault("GMC_JWT_SECRET", "dev-secret-change-me"),
		DBPath:             getOrDefault("GMC_DB_PATH", "data/gomeshcentral.db"),
		BootstrapAdminUser: getOrDefault("GMC_BOOTSTRAP_ADMIN_USER", "admin"),
		BootstrapAdminPass: getOrDefault("GMC_BOOTSTRAP_ADMIN_PASS", "admin123!"),
	}
	return cfg
}

func getOrDefault(key, fallback string) string {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	return v
}
