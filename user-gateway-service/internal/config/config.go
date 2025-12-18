package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

// MustLoadConfig loads configuration from file and environment
func MustLoadConfig(path string) error {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	if path == "" {
		path = "./config"
	}
	viper.AddConfigPath(path)
	if err := viper.ReadInConfig(); err != nil {
		return fmt.Errorf("error reading config file: %w", err)
	}
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()
	return nil
}

// Config holds application configuration
type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	Services ServicesConfig `mapstructure:"services"`
	ENV      string         `mapstructure:"env"`
}

// ServerConfig holds server configuration
type ServerConfig struct {
	Host string `mapstructure:"host"`
	Port string `mapstructure:"port"`
}

// ServicesConfig holds external services configuration
type ServicesConfig struct {
	PRAllocation ServiceConfig `mapstructure:"pr_allocation"`
	CodeStorage  ServiceConfig `mapstructure:"code_storage"`
}

// ServiceConfig holds single service configuration
type ServiceConfig struct {
	Host string `mapstructure:"host"`
	Port string `mapstructure:"port"`
}

// GetConfig returns the config struct populated from viper
func GetConfig() (*Config, error) {
	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unable to decode config into struct: %w", err)
	}
	return &cfg, nil
}

// GetPRAllocationURL returns the full URL for PR allocation service
func (c *Config) GetPRAllocationURL() string {
	return fmt.Sprintf("http://%s:%s", c.Services.PRAllocation.Host, c.Services.PRAllocation.Port)
}

// GetCodeStorageURL returns the full URL for code storage service
func (c *Config) GetCodeStorageURL() string {
	return fmt.Sprintf("http://%s:%s", c.Services.CodeStorage.Host, c.Services.CodeStorage.Port)
}
