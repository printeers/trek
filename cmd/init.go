package cmd

import (
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
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			initializeConfig(cmd)

			return nil
		},
		Run: func(cmd *cobra.Command, args []string) {
			var err error

			if version == "" {
				trekVersionPrompt := promptui.Prompt{
					Label:   "Trek version",
					Default: "latest", //nolint:lll,godox // TODO: When tagging a release in the CI, the version should be injected into the go program so we can set it as the default value here
				}
				version, err = trekVersionPrompt.Run()
				if err != nil {
					log.Fatalln(err)
				}
			}

			if modelName == "" || databaseName == "" || databaseUsers == "" {
				fmt.Printf("The following answers can only contain a-z and _\n")
			}

			if modelName != "" {
				if err = validateModelName(modelName); err != nil {
					log.Fatalln(err)
				}
			} else {
				modelNamePrompt := promptui.Prompt{
					Label:    "Model name",
					Validate: validateModelName,
				}
				modelName, err = modelNamePrompt.Run()
				if err != nil {
					log.Fatalln(err)
				}
			}

			if databaseName != "" {
				if err = validateDatabaseName(databaseName); err != nil {
					log.Fatalln(err)
				}
			} else {
				dbNamePrompt := promptui.Prompt{
					Label:    "Database name",
					Validate: validateDatabaseName,
				}
				databaseName, err = dbNamePrompt.Run()
				if err != nil {
					log.Fatalln(err)
				}
			}

			if databaseUsers != "" {
				if err = validateDatabaseUsers(databaseUsers); err != nil {
					fmt.Println("bla")
					log.Fatalln(err)
				}
			} else {
				dbUsersPrompt := promptui.Prompt{
					Label:    "Database users (comma separated)",
					Validate: validateDatabaseUsers,
				}
				databaseUsers, err = dbUsersPrompt.Run()
				if err != nil {
					log.Fatalln(err)
				}
			}

			templateData := map[string]interface{}{
				"trek_version": version,
				"model_name":   modelName,
				"db_name":      databaseName,
				"db_users":     strings.Split(databaseUsers, ","),
			}

			err = writeTemplateFile(embed.DbmTmpl, fmt.Sprintf("%s.dbm", modelName), templateData)
			if err != nil {
				log.Fatalln(err)
			}
			err = writeTemplateFile(embed.DockerComposeYamlTmpl, "docker-compose.yaml", templateData)
			if err != nil {
				log.Fatalln(err)
			}
			err = writeTemplateFile(embed.DockerfileTmpl, "Dockerfile", templateData)
			if err != nil {
				log.Fatalln(err)
			}
			err = writeTemplateFile(embed.TrekYamlTmpl, "trek.yaml", templateData)
			if err != nil {
				log.Fatalln(err)
			}

			err = os.MkdirAll("migrations", 0o755)
			if err != nil {
				log.Fatalln(err)
			}
			err = os.MkdirAll("testdata", 0o755)
			if err != nil {
				log.Fatalln(err)
			}

			_, err = os.Create("testdata/001_0101-content.sql")
			if err != nil {
				log.Fatalln(err)
			}

			log.Println("New project created!")

			config, err := internal.ReadConfig()
			if err != nil {
				log.Fatalf("Failed to read config: %v\n", err)
			}

			wd, wdErr := os.Getwd()
			if wdErr != nil {
				log.Fatalf("Failed to get working directory: %v\n", wdErr)
			}

			err = runWithFile(config, filepath.Join(wd, "migrations", "001_init.up.sql"), 1)
			if err != nil {
				log.Fatalln(err)
			}
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
		return fmt.Errorf("failed to create file %s: %w", filename, err)
	}
	err = t.Execute(f, templateData)
	if err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	return nil
}
