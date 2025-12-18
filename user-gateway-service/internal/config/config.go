package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/viper"
)

// MustLoadConfig loads configuration from file and environment
func MustLoadConfig(path string) error {
	// Set default values
	viper.SetDefault("server.host", "0.0.0.0")
	viper.SetDefault("server.port", "8082")
	viper.SetDefault("services.pr_allocation.host", "localhost")
	viper.SetDefault("services.pr_allocation.port", "8080")
	viper.SetDefault("services.code_storage.host", "localhost")
	viper.SetDefault("services.code_storage.port", "8081")
	viper.SetDefault("env", "development")

	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	if path == "" {
		path = "./config"
	}
	viper.AddConfigPath(path)

	// Try to read config file, but don't fail if not found
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return fmt.Errorf("error reading config file: %w", err)
		}
	}

	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	// Explicit environment variable bindings
	bindEnvWithDefault("server.host", "SERVER_HOST")
	bindEnvWithDefault("server.port", "SERVER_PORT")
	bindEnvWithDefault("services.pr_allocation.host", "PR_ALLOCATION_HOST")
	bindEnvWithDefault("services.pr_allocation.port", "PR_ALLOCATION_PORT")
	bindEnvWithDefault("services.code_storage.host", "CODE_STORAGE_HOST")
	bindEnvWithDefault("services.code_storage.port", "CODE_STORAGE_PORT")
	bindEnvWithDefault("env", "ENV")

	// Support full URL env vars
	if url := os.Getenv("PR_ALLOCATION_SERVICE_URL"); url != "" {
		// Parse URL and set host/port
		viper.Set("services.pr_allocation.url", url)
	}
	if url := os.Getenv("CODE_STORAGE_SERVICE_URL"); url != "" {
		viper.Set("services.code_storage.url", url)
	}

	return nil
}

func bindEnvWithDefault(key, envVar string) {
	if val := os.Getenv(envVar); val != "" {
		viper.Set(key, val)
	}
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
