package config

import "fmt"

type Config struct {
	DBHost     string
	DBPort     int
	DBUser     string
	DBPassword string
	DBName     string
	DBSSLMode  string
	DBTimeZone string
}

func (c Config) PostgresDSN() string {
	sslMode := c.DBSSLMode
	if sslMode == "" {
		sslMode = "disable"
	}

	timeZone := c.DBTimeZone
	if timeZone == "" {
		timeZone = "UTC"
	}

	port := c.DBPort
	if port == 0 {
		port = 5432
	}

	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s TimeZone=%s",
		c.DBHost,
		port,
		c.DBUser,
		c.DBPassword,
		c.DBName,
		sslMode,
		timeZone,
	)
}
