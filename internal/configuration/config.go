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
	Output    *Output    `yaml:"output"`
}

type Role struct {
	Name string `yaml:"name"`
}

type Template struct {
	Path    string `yaml:"path"`
	Content string `yaml:"content"`
}

type OutputFile struct {
	Path string `yaml:"path"`
}

type Output struct {
	SQL *OutputFile `yaml:"sql"`
	PNG *OutputFile `yaml:"png"`
	SVG *OutputFile `yaml:"svg"`
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

// GetOutputPath returns the output path for the given type if enabled, or empty string if not.
// The outputType must be one of: "sql", "png", "svg". Panics if an invalid outputType is provided.
func (c *Config) GetOutputPath(outputType string) string {
	if c.Output == nil {
		return ""
	}

	var outputFile *OutputFile

	switch outputType {
	case "sql":
		outputFile = c.Output.SQL
	case "png":
		outputFile = c.Output.PNG
	case "svg":
		outputFile = c.Output.SVG
	default:
		panic(fmt.Sprintf("invalid output type: %q", outputType))
	}

	if outputFile == nil {
		return ""
	}

	if outputFile.Path != "" {
		return outputFile.Path
	}

	// Default path: {model_name}.gen.{ext}
	return fmt.Sprintf("%s.gen.%s", c.ModelName, outputType)
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
