package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

var writeConfigCmd = &cobra.Command{
	Use:   "writeConfig",
	Short: "Write config to file",
	Run: func(cmd *cobra.Command, args []string) {
		outFile := cmd.Flag("output").Value.String()

		if outFile == "" {
			settings := viper.AllSettings()
			yamlSettings, err := yaml.Marshal(settings)
			if err != nil {
				fmt.Fprintln(cmd.OutOrStderr(), err)
				os.Exit(1)
			}

			fmt.Fprintln(cmd.OutOrStdout(), string(yamlSettings))
		} else {
			if err := viper.SafeWriteConfigAs(outFile); err != nil {
				fmt.Fprintln(cmd.OutOrStderr(), err)
				os.Exit(1)
			}
		}
	},
}

func init() {
	rootCmd.AddCommand(writeConfigCmd)

	writeConfigCmd.Flags().StringP("output", "o", "", "File to write config to (default is stdout)")
}
