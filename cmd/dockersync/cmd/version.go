package cmd

import (
	"fmt"

	"github.com/Altinity/docker-sync/config"
	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Gets chguard version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(config.Version)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
