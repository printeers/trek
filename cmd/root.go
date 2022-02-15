package cmd

import "github.com/spf13/cobra"

func NewRootCommand() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "trek",
		Short: "A tool to do automatic database migrations created by Stack11",
	}

	rootCmd.AddCommand(NewApplyCommand())
	rootCmd.AddCommand(NewGenerateCommand())
	rootCmd.AddCommand(NewInitCommand())

	return rootCmd
}
