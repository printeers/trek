package cmd

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"text/template"

	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
	"github.com/thecodeteam/goodbye"

	"github.com/stack11/trek/internal"
	"github.com/stack11/trek/internal/embed"
)

var errInvalidModelName = errors.New("invalid model name")
var errInvalidDatabaseName = errors.New("invalid database name")
var errInvalidDatabaseUsersList = errors.New("invalid database users list")

//nolint:gochecknoglobals
var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Create a new trek project",
	Run: func(cmd *cobra.Command, args []string) {
		var err error

		trekVersion := os.Getenv("TREK_VERSION")
		modelName := os.Getenv("MODEL_NAME")
		dbName := os.Getenv("DATABASE_NAME")
		dbUsers := os.Getenv("DATABASE_USERS")

		if trekVersion == "" {
			trekVersionPrompt := promptui.Prompt{
				Label:   "Trek version",
				Default: "latest", //nolint:lll,godox // TODO: When tagging a release in the CI, the version should be injected into the go program so we can set it as the default value here
			}
			trekVersion, err = trekVersionPrompt.Run()
			if err != nil {
				log.Fatalln(err)
			}
		}

		if modelName == "" || dbName == "" || dbUsers == "" {
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

		if dbName != "" {
			if err = validateDatabaseName(dbName); err != nil {
				log.Fatalln(err)
			}
		} else {
			dbNamePrompt := promptui.Prompt{
				Label:    "Database name",
				Validate: validateDatabaseName,
			}
			dbName, err = dbNamePrompt.Run()
			if err != nil {
				log.Fatalln(err)
			}
		}

		if dbUsers != "" {
			if err = validateDatabaseUsers(dbUsers); err != nil {
				log.Fatalln(err)
			}
		} else {
			dbUsersPrompt := promptui.Prompt{
				Label:    "Database users (comma separated)",
				Validate: validateDatabaseUsers,
			}
			dbUsers, err = dbUsersPrompt.Run()
			if err != nil {
				log.Fatalln(err)
			}
		}

		templateData := map[string]interface{}{
			"trek_version": trekVersion,
			"model_name":   modelName,
			"db_name":      dbName,
			"db_users":     strings.Split(dbUsers, ","),
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

		internal.AssertGenerateToolsAvailable()

		config, err := internal.ReadConfig()
		if err != nil {
			log.Fatalf("Failed to read config: %v\n", err)
		}

		ctx := context.Background()
		defer goodbye.Exit(ctx, -1)
		goodbye.Notify(ctx)
		goodbye.Register(func(ctx context.Context, sig os.Signal) {
			internal.DockerKillContainer(targetContainerID)
			internal.DockerKillContainer(migrateContainerID)
		})

		updateDiff(*config, "", true)
	},
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
