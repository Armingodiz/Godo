package config

import (
	"os"
	"strconv"
)

type Config struct {
	App   AppConfig
	DB    DatabaseConfig
	Redis RedisConfig
	AWS   AWSConfig
}

type AppConfig struct {
	Port string
}

type DatabaseConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	Name     string
}

type RedisConfig struct {
	Host     string
	Port     string
	DB       int
	Password string
}

type AWSConfig struct {
	Endpoint string
	Region   string
	S3Bucket string
}

func Load() *Config {
	return &Config{
		App: AppConfig{
			Port: getEnv("PORT", "8083"),
		},
		DB: DatabaseConfig{
			Host:     getEnv("DB_HOST", "localhost"),
			Port:     getEnv("DB_PORT", "3306"),
			User:     getEnv("DB_USER", "todo_user"),
			Password: getEnv("DB_PASSWORD", "todo_password"),
			Name:     getEnv("DB_NAME", "todo_db"),
		},
		Redis: RedisConfig{
			Host:     getEnv("REDIS_HOST", "localhost"),
			Port:     getEnv("REDIS_PORT", "6379"),
			DB:       getIntEnv("REDIS_DB", 0),
			Password: getEnv("REDIS_PASSWORD", ""),
		},
		AWS: AWSConfig{
			Endpoint: getEnv("AWS_ENDPOINT_URL", "http://localhost:4566"),
			Region:   getEnv("AWS_REGION", "us-east-1"),
			S3Bucket: getEnv("S3_BUCKET", "todo-bucket"),
		},
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getIntEnv(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}
