package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/viper"
)

func MustLoadConfig(path string) error {
	// Set default values
	viper.SetDefault("server.host", "0.0.0.0")
	viper.SetDefault("server.port", "8080")
	viper.SetDefault("database.host", "localhost")
	viper.SetDefault("database.port", "5432")
	viper.SetDefault("database.user", "postgres")
	viper.SetDefault("database.password", "postgres")
	viper.SetDefault("database.dbname", "pr_allocation")
	viper.SetDefault("database.sslmode", "disable")
	viper.SetDefault("env", "development")

	// reading from YAML
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	if path == "" {
		path = "./config"
	}
	viper.AddConfigPath(path)

	// Try to read config file, but don't fail if not found
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return fmt.Errorf("error reading config file, %w", err)
		}
	}

	// override through .env if presents
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	// Explicit environment variable bindings
	bindEnvWithDefault("server.host", "SERVER_HOST")
	bindEnvWithDefault("server.port", "SERVER_PORT")
	bindEnvWithDefault("database.host", "DB_HOST")
	bindEnvWithDefault("database.port", "DB_PORT")
	bindEnvWithDefault("database.user", "DB_USER")
	bindEnvWithDefault("database.password", "DB_PASSWORD")
	bindEnvWithDefault("database.dbname", "DB_NAME")
	bindEnvWithDefault("database.sslmode", "DB_SSLMODE")
	bindEnvWithDefault("env", "ENV")

	return nil
}

func bindEnvWithDefault(key, envVar string) {
	if val := os.Getenv(envVar); val != "" {
		viper.Set(key, val)
	}
}

type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	Database DatabaseConfig `mapstructure:"database"`
	ENV      string         `mapstructure:"env"`
}

type ServerConfig struct {
	Host string `mapstructure:"host"`
	Port string `mapstructure:"port"`
}

type DatabaseConfig struct {
	Host     string `mapstructure:"host"`
	Port     string `mapstructure:"port"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	DBName   string `mapstructure:"dbname"`
	SSLMode  string `mapstructure:"sslmode"`
}

// GetConfig returns the config struct populated from viper.
func GetConfig() (*Config, error) {
	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unable to decode config into struct: %w", err)
	}
	return &cfg, nil
}
