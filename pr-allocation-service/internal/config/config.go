package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

func MustLoadConfig(path string) error {
	// reading from YAML
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	if path == "" {
		path = "./config"
	}
	viper.AddConfigPath(path)
	if err := viper.ReadInConfig(); err != nil {
		return fmt.Errorf("error reading config file, %w", err)
	}
	// override through .env if presents
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()
	return nil
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
