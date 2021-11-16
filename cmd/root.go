package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

//nolint:gochecknoglobals
var rootCmd = &cobra.Command{
	Use:   "trek",
	Short: "A tool to do automatic database migrations created by Stack11",
}

func Execute() {
	rootCmd.AddCommand(applyCmd)
	generateCmd.Flags().BoolVarP(
		&flagDiffInitial,
		"initial",
		"i",
		false,
		"Directly copy the diff to the migrations. Used for first time setup",
	)
	generateCmd.Flags().BoolVar(
		&flagOnce,
		"once",
		false,
		"Run only once and don't watch files",
	)
	rootCmd.AddCommand(generateCmd)
	if err := rootCmd.Execute(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
