package config

import "os"

type Config struct {
	Addr        string
	DatabaseURL string
	StaticDir   string
}

func Load() Config {
	return Config{
		Addr:        addr(),
		DatabaseURL: env("DATABASE_URL", "postgres://ledger:ledger@localhost:5432/ledger?sslmode=disable"),
		StaticDir:   os.Getenv("STATIC_DIR"),
	}
}

// addr honors PORT when a host sets it, otherwise LEDGER_ADDR.
func addr() string {
	if p := os.Getenv("PORT"); p != "" {
		return ":" + p
	}
	return env("LEDGER_ADDR", ":8080")
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
