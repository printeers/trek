package cmd

import (
	"fmt"
	"log"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

const envPrefix = "TREK"

func initializeConfig(cmd *cobra.Command) {
	v := viper.New()
	v.SetEnvPrefix(envPrefix)
	v.AutomaticEnv()
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		if strings.Contains(f.Name, "-") {
			_ = v.BindEnv(f.Name, fmt.Sprintf("%s_%s", envPrefix, strings.ToUpper(strings.ReplaceAll(f.Name, "-", "_"))))
		}
		if !f.Changed && v.IsSet(f.Name) {
			val := v.Get(f.Name)
			_ = cmd.Flags().Set(f.Name, fmt.Sprintf("%v", val))
		}
	})
}

func markFlagRequired(cmd *cobra.Command, flag string) {
	err := cmd.MarkFlagRequired(flag)
	if err != nil {
		log.Fatalf("Failed to mark flag %s as required: %v\n", flag, err)
	}
}
