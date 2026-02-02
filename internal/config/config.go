package config

import (
	"os"
	"strconv"
)

const (
	DefaultHost                = "0.0.0.0"
	DefaultPort                = "3002"
	DefaultCountryDBPath       = "/data/country.mmdb"
	DefaultCountryDBURL        = "https://cdn.jsdelivr.net/npm/@ip-location-db/geolite2-geo-whois-asn-country-mmdb/geolite2-geo-whois-asn-country.mmdb"
	DefaultCityDBIPv4Path      = "/data/city-ipv4.mmdb"
	DefaultCityDBIPv4URL       = "https://cdn.jsdelivr.net/npm/@ip-location-db/geolite2-city-mmdb/geolite2-city-ipv4.mmdb"
	DefaultCityDBIPv6Path      = "/data/city-ipv6.mmdb"
	DefaultCityDBIPv6URL       = "https://cdn.jsdelivr.net/npm/@ip-location-db/geolite2-city-mmdb/geolite2-city-ipv6.mmdb"
	DefaultUpdateIntervalHours = 24
)

type Config struct {
	Host                string
	Port                string
	CountryDBPath    string
	CountryDBURL     string
	CityDBIPv4Path   string
	CityDBIPv4URL    string
	CityDBIPv6Path   string
	CityDBIPv6URL    string
	UpdateIntervalHours int
	APIKey              string
}

func Load() *Config {
	return &Config{
		Host:                getEnv("HOST", DefaultHost),
		Port:                getEnv("PORT", DefaultPort),
		CountryDBPath:    getEnv("COUNTRY_DB_PATH", DefaultCountryDBPath),
		CountryDBURL:     getEnv("COUNTRY_DB_URL", DefaultCountryDBURL),
		CityDBIPv4Path:   getEnv("CITY_DB_IPV4_PATH", DefaultCityDBIPv4Path),
		CityDBIPv4URL:    getEnv("CITY_DB_IPV4_URL", DefaultCityDBIPv4URL),
		CityDBIPv6Path:   getEnv("CITY_DB_IPV6_PATH", DefaultCityDBIPv6Path),
		CityDBIPv6URL:    getEnv("CITY_DB_IPV6_URL", DefaultCityDBIPv6URL),
		UpdateIntervalHours: getEnvInt("UPDATE_INTERVAL_HOURS", DefaultUpdateIntervalHours),
		APIKey:              os.Getenv("API_KEY"),
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
