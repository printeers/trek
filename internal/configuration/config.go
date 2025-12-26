package configuration

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"gopkg.in/yaml.v2"
)

const regexpStringValidIdentifier = `^[a-z_]+$`

var regexpValidIdentifier = regexp.MustCompile(regexpStringValidIdentifier)

var ErrInvalidValuesInConfig = errors.New("invalid values in config")

type Config struct {
	//nolint:tagliatelle
	ModelName string `yaml:"model_name"`
	//nolint:tagliatelle
	DatabaseName string `yaml:"db_name"`
	//nolint:tagliatelle
	Roles     []Role     `yaml:"roles"`
	Templates []Template `yaml:"templates"`
}
type Role struct {
	Name string `yaml:"name"`
}

type Template struct {
	Path    string `yaml:"path"`
	Content string `yaml:"content"`
}

func ReadConfig(wd string) (*Config, error) {
	var config *Config
	file, err := os.ReadFile(filepath.Join(wd, "trek.yaml"))
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}
	err = yaml.Unmarshal(file, &config)
	if err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	problems := config.validate()
	if len(problems) > 0 {
		for _, problem := range problems {
			fmt.Println("Error in trek.yaml: " + problem)
		}

		return nil, ErrInvalidValuesInConfig
	}

	return config, nil
}

func (c *Config) validate() (problems []string) {
	if !ValidateIdentifier(c.ModelName) {
		p := fmt.Sprintf("Model name %q contains invalid characters. Must match %q.",
			c.ModelName,
			regexpStringValidIdentifier,
		)
		problems = append(problems, p)
	}
	if !ValidateIdentifier(c.DatabaseName) {
		p := fmt.Sprintf("Database name %q contains invalid characters. Must match %q.",
			c.DatabaseName,
			regexpStringValidIdentifier,
		)
		problems = append(problems, p)
	}
	for _, role := range c.Roles {
		if !ValidateIdentifier(role.Name) {
			p := fmt.Sprintf("Database user %q contains invalid characters. Must match %q.",
				role,
				regexpStringValidIdentifier,
			)
			problems = append(problems, p)
		}
	}

	return problems
}

func ValidateIdentifier(identifier string) bool {
	return regexpValidIdentifier.MatchString(identifier)
}

func ValidateIdentifierList(identifiers []string) bool {
	valid := true
	for _, identifier := range identifiers {
		if !regexpValidIdentifier.MatchString(identifier) {
			valid = false
		}
	}

	return valid
}
