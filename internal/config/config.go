package config

import (
	"os"
)

type Config struct {
	ListenAddr        string
	DBPath            string
	StoragePath       string
	ImageAllowance    int
	AuthToken         string
	BaseURL           string
	EnforceSignedURLs bool
}

func Load() *Config {
	return &Config{
		ListenAddr:     getEnv("DT_LISTEN_ADDR", ":8080"),
		DBPath:         getEnv("DT_DB_PATH", "/data/db/images.db"),
		StoragePath:    getEnv("DT_STORAGE_PATH", "/data/images"),
		ImageAllowance: getEnvInt("DT_IMAGE_ALLOWANCE", 100000),
		AuthToken:      getEnv("DT_AUTH_TOKEN", ""),
		BaseURL:           getEnv("DT_BASE_URL", "http://localhost:8080"),
		EnforceSignedURLs: getEnv("DT_ENFORCE_SIGNED_URLS", "") == "true",
	}
}

func getEnv(key, defaultValue string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	v := os.Getenv(key)
	if v == "" {
		return defaultValue
	}
	var result int
	for _, c := range v {
		if c < '0' || c > '9' {
			return defaultValue
		}
		result = result*10 + int(c-'0')
	}
	return result
}
