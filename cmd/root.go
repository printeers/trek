package cmd

import (
	"github.com/spf13/cobra"

	"github.com/printeers/trek/internal"
	"github.com/printeers/trek/internal/embedded/migra"
)

func NewRootCommand() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "trek",
		Short: "A tool to create, organize and run database migrations.",
		PersistentPreRun: func(cmd *cobra.Command, _ []string) {
			internal.InitializeFlags(cmd)
		},
	}

	rootCmd.PersistentFlags().BoolVar(
		&migra.ForceEmbedded,
		"force-embedded-migra", false,
		"Force using the embedded migra binary instead of the system one.")
	rootCmd.AddCommand(NewApplyCommand())
	rootCmd.AddCommand(NewCheckCommand())
	rootCmd.AddCommand(NewGenerateCommand())
	rootCmd.AddCommand(NewInitCommand())

	return rootCmd
}
