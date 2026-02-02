package config

import (
	"os"
	"strconv"
)

const (
	DefaultHost                = "0.0.0.0"
	DefaultPort                = "3002"
	DefaultMMDBPath            = "/data/geo.mmdb"
	DefaultMMDBURL             = "https://cdn.jsdelivr.net/npm/@ip-location-db/geolite2-geo-whois-asn-country-mmdb/geolite2-geo-whois-asn-country.mmdb"
	DefaultUpdateIntervalHours = 24
)

type Config struct {
	Host                string
	Port                string
	MMDBPath            string
	MMDBURL             string
	UpdateIntervalHours int
	APIKey              string
}

func Load() *Config {
	return &Config{
		Host:                getEnv("HOST", DefaultHost),
		Port:                getEnv("PORT", DefaultPort),
		MMDBPath:            getEnv("MMDB_PATH", DefaultMMDBPath),
		MMDBURL:             getEnv("MMDB_URL", DefaultMMDBURL),
		UpdateIntervalHours: getEnvInt("UPDATE_INTERVAL_HOURS", DefaultUpdateIntervalHours),
		APIKey:              os.Getenv("API_KEY"), // empty = no auth required
	}
}

func (c *Config) Addr() string {
	return c.Host + ":" + c.Port
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}
