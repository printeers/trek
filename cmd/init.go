package cmd

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
	"github.com/stack11/trek/internal"
	"github.com/stack11/trek/internal/embed"
)

var errInvalidModelName = errors.New("invalid model name")
var errInvalidDatabaseName = errors.New("invalid database name")
var errInvalidDatabaseUsersList = errors.New("invalid database users list")

//nolint:gocognit,cyclop
func NewInitCommand() *cobra.Command {
	var (
		version       string
		modelName     string
		databaseName  string
		databaseUsers string
	)
	initCmd := &cobra.Command{
		Use:   "init",
		Short: "Create a new trek project",
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			internal.InitializeFlags(cmd)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()

			var err error

			if version == "" {
				trekVersionPrompt := promptui.Prompt{
					Label:   "Trek version",
					Default: "latest", //nolint:lll,godox // TODO: When tagging a release in the CI, the version should be injected into the go program so we can set it as the default value here
				}
				version, err = trekVersionPrompt.Run()
				if err != nil {
					return fmt.Errorf("failed to prompt version: %w", err)
				}
			}

			if modelName == "" || databaseName == "" || databaseUsers == "" {
				fmt.Printf("The following answers can only contain a-z and _\n")
			}

			if modelName != "" {
				if err = validateModelName(modelName); err != nil {
					return fmt.Errorf("invalid model name %q: %w", modelName, err)
				}
			} else {
				modelNamePrompt := promptui.Prompt{
					Label:    "Model name",
					Validate: validateModelName,
				}
				modelName, err = modelNamePrompt.Run()
				if err != nil {
					return fmt.Errorf("failed to prompt model name: %w", err)
				}
			}

			if databaseName != "" {
				if err = validateDatabaseName(databaseName); err != nil {
					return fmt.Errorf("invalid database name %q: %w", databaseName, err)
				}
			} else {
				dbNamePrompt := promptui.Prompt{
					Label:    "Database name",
					Validate: validateDatabaseName,
				}
				databaseName, err = dbNamePrompt.Run()
				if err != nil {
					return fmt.Errorf("failed to prompt database name: %w", err)
				}
			}

			if databaseUsers != "" {
				if err = validateDatabaseUsers(databaseUsers); err != nil {
					return fmt.Errorf("invalid database users %q: %w", databaseUsers, err)
				}
			} else {
				dbUsersPrompt := promptui.Prompt{
					Label:    "Database users (comma separated)",
					Validate: validateDatabaseUsers,
				}
				databaseUsers, err = dbUsersPrompt.Run()
				if err != nil {
					return fmt.Errorf("failed to prompt database users: %w", err)
				}
			}

			templateData := map[string]interface{}{
				"trek_version": version,
				"model_name":   modelName,
				"db_name":      databaseName,
				"db_users":     strings.Split(databaseUsers, ","),
			}

			for file, tmpl := range map[string]string{
				fmt.Sprintf("%s.dbm", modelName): embed.DbmTmpl,
				"docker-compose.yaml":            embed.DockerComposeYamlTmpl,
				"Dockerfile":                     embed.DockerfileTmpl,
				"trek.yaml":                      embed.TrekYamlTmpl,
			} {
				err = writeTemplateFile(tmpl, file, templateData)
				if err != nil {
					return fmt.Errorf("failed to write %q: %w", file, err)
				}
			}

			for _, dir := range []string{"migrations", "testdata", "hooks"} {
				err = os.MkdirAll(dir, 0o755)
				if err != nil {
					return fmt.Errorf("failed to create directory %q: %w", dir, err)
				}
			}

			_, err = os.Create("testdata/001_0101-content.sql")
			if err != nil {
				return fmt.Errorf("failed to create testdata file: %w", err)
			}

			for name, args := range map[string][]string{
				"apply-reset-pre":         {},
				"apply-reset-post":        {},
				"generate-migration-post": {"echo \"Running on migration file $1\""},
			} {
				err = writeSampleHook(name, args...)
				if err != nil {
					return fmt.Errorf("failed to write hook %q: %w", name, err)
				}
			}

			log.Println("New project created!")

			config, err := internal.ReadConfig()
			if err != nil {
				return fmt.Errorf("failed to read config: %w", err)
			}

			wd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("failed to get working directory: %w", err)
			}

			err = runWithFile(ctx, config, filepath.Join(wd, "migrations", "001_init.up.sql"), 1)
			if err != nil {
				return fmt.Errorf("failed to generate first migration: %w", err)
			}

			return nil
		},
	}

	initCmd.Flags().StringVar(&version, "version", "", "Trek version to use (in the Dockerfile)")
	initCmd.Flags().StringVar(&modelName, "model-name", "", "Model (file) name")
	initCmd.Flags().StringVar(&databaseName, "database-name", "", "Database name")
	initCmd.Flags().StringVar(&databaseUsers, "database-users", "", "Database users")

	return initCmd
}

func validateModelName(s string) error {
	if !internal.ValidateIdentifier(s) {
		return errInvalidModelName
	}

	return nil
}

func validateDatabaseName(s string) error {
	if !internal.ValidateIdentifier(s) {
		return errInvalidDatabaseName
	}

	return nil
}

func validateDatabaseUsers(s string) error {
	if !internal.ValidateIdentifierList(strings.Split(s, ",")) {
		return errInvalidDatabaseUsersList
	}

	return nil
}

func writeTemplateFile(ts, filename string, templateData map[string]interface{}) error {
	t, err := template.New(filename).Parse(ts)
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}
	f, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create file %q: %w", filename, err)
	}
	err = t.Execute(f, templateData)
	if err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	return nil
}

func writeSampleHook(name string, extraLines ...string) error {
	lines := []string{
		"#!/bin/bash",
		"set -euxo pipefail",
		"",
		fmt.Sprintf("echo \"This is %s\"", name),
	}
	if len(extraLines) > 0 {
		lines = append(lines, extraLines...)
	}

	lines = append(lines, "")

	//nolint:gosec,wrapcheck
	return os.WriteFile(fmt.Sprintf("hooks/%s.sample", name), []byte(strings.Join(lines, "\n")), 0o755)
}
