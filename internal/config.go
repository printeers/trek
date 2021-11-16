package internal

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v2"
)

type Config struct {
	//nolint:tagliatelle
	ModelName string `yaml:"model_name"`
	//nolint:tagliatelle
	DatabaseName string `yaml:"db_name"`
	//nolint:tagliatelle
	DatabaseUsers []string `yaml:"db_users"`
}

func ReadConfig() (*Config, error) {
	var config *Config
	file, err := os.ReadFile("config.yaml")
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}
	err = yaml.Unmarshal(file, &config)
	if err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return config, nil
}
