package cmd

import "github.com/spf13/cobra"

func NewRootCommand() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "trek",
		Short: "A tool to create, organize and run database migrations.",
	}

	rootCmd.AddCommand(NewApplyCommand())
	rootCmd.AddCommand(NewCheckCommand())
	rootCmd.AddCommand(NewGenerateCommand())
	rootCmd.AddCommand(NewInitCommand())

	return rootCmd
}
