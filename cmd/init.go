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

	"github.com/printeers/trek/internal"
	"github.com/printeers/trek/internal/configuration"
	"github.com/printeers/trek/internal/templates"
)

var errInvalidModelName = errors.New("invalid model name")
var errInvalidDatabaseName = errors.New("invalid database name")
var errInvalidRolesList = errors.New("invalid roles list")

//nolint:gocognit,cyclop
func NewInitCommand() *cobra.Command {
	var (
		version      string
		modelName    string
		databaseName string
		roleNames    string
	)
	initCmd := &cobra.Command{
		Use:   "init",
		Short: "Create a new trek project",
		PersistentPreRun: func(cmd *cobra.Command, _ []string) {
			internal.InitializeFlags(cmd)
		},
		RunE: func(_ *cobra.Command, _ []string) error {
			ctx := context.Background()

			wd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("failed to get working directory: %w", err)
			}

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

			if modelName == "" || databaseName == "" || roleNames == "" {
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

			if roleNames != "" {
				if err = validateRoles(roleNames); err != nil {
					return fmt.Errorf("invalid roles %q: %w", roleNames, err)
				}
			} else {
				dbUsersPrompt := promptui.Prompt{
					Label:    "Roles (comma separated)",
					Validate: validateRoles,
				}
				roleNames, err = dbUsersPrompt.Run()
				if err != nil {
					return fmt.Errorf("failed to prompt roles: %w", err)
				}
			}

			templateData := map[string]any{
				"trek_version": version,
				"model_name":   modelName,
				"db_name":      databaseName,
				"roleNames":    strings.Split(roleNames, ","),
			}

			for file, tmpl := range map[string]string{
				fmt.Sprintf("%s.dbm", modelName): templates.DbmTmpl,
				"docker-compose.yaml":            templates.DockerComposeYamlTmpl,
				"Dockerfile":                     templates.DockerfileTmpl,
				"trek.yaml":                      templates.TrekYamlTmpl,
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

			_, err = os.Create(filepath.Join(wd, "testdata", "001_0101-content.sql"))
			if err != nil {
				return fmt.Errorf("failed to create testdata file: %w", err)
			}

			for name, args := range map[string][]string{
				"apply-reset-pre":         {},
				"apply-reset-post":        {},
				"generate-migration-post": {"echo \"Running on migration file $1\""},
			} {
				err = writeSampleHook(wd, name, args...)
				if err != nil {
					return fmt.Errorf("failed to write hook %q: %w", name, err)
				}
			}

			log.Println("New project created!")

			config, err := configuration.ReadConfig(wd)
			if err != nil {
				return fmt.Errorf("failed to read config: %w", err)
			}

			migrationsDir, err := internal.GetMigrationsDir(wd)
			if err != nil {
				return fmt.Errorf("failed to get migrations directory: %w", err)
			}

			tmpDir, err := os.MkdirTemp("", "trek-")
			if err != nil {
				return fmt.Errorf("failed to create temporary directory: %w", err)
			}

			_, err = runWithFile(
				ctx,
				config,
				wd,
				tmpDir,
				migrationsDir,
				filepath.Join(migrationsDir, "001_init.up.sql"),
				1,
				false,
			)
			if err != nil {
				return fmt.Errorf("failed to generate first migration: %w", err)
			}

			//nolint:wrapcheck
			return os.RemoveAll(tmpDir)
		},
	}

	initCmd.Flags().StringVar(&version, "version", "", "Trek version to use (in the Dockerfile)")
	initCmd.Flags().StringVar(&modelName, "model-name", "", "Model (file) name")
	initCmd.Flags().StringVar(&databaseName, "database-name", "", "Database name")
	initCmd.Flags().StringVar(&roleNames, "roles", "", "Roles")

	return initCmd
}

func validateModelName(s string) error {
	if !configuration.ValidateIdentifier(s) {
		return errInvalidModelName
	}

	return nil
}

func validateDatabaseName(s string) error {
	if !configuration.ValidateIdentifier(s) {
		return errInvalidDatabaseName
	}

	return nil
}

func validateRoles(s string) error {
	if !configuration.ValidateIdentifierList(strings.Split(s, ",")) {
		return errInvalidRolesList
	}

	return nil
}

func writeTemplateFile(ts, filename string, templateData map[string]any) error {
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

func writeSampleHook(wd, name string, extraLines ...string) error {
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
	return os.WriteFile(
		filepath.Join(wd, "hooks", fmt.Sprintf("%s.sample", name)),
		[]byte(strings.Join(lines, "\n")),
		0o755,
	)
}
