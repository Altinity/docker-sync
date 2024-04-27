package cmd

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	dockersync "github.com/Altinity/docker-sync"
	"github.com/Altinity/docker-sync/config"
	"github.com/Altinity/docker-sync/logging"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

var cfgFile string

var rootCmd = &cobra.Command{
	Use:   "dockersync",
	Short: "Keep your Docker images in sync",
	PreRun: func(cmd *cobra.Command, args []string) {
		cmd.Annotations = make(map[string]string)
		cmd.Annotations["error"] = ""
	},
	Run: func(cmd *cobra.Command, args []string) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		c := make(chan os.Signal, 1)
		signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
		go func() {
			<-c
			cancel()
			time.Sleep(1 * time.Second)
			os.Exit(0)
		}()

		logging.ReloadGlobalLogger()

		log.Info().Msg("Starting Docker Sync")

		if err := dockersync.Run(ctx); err != nil {
			cmd.Annotations["error"] = err.Error()
			log.Error().
				Stack().
				Err(err).
				Msg("Error running Docker Sync")
		}

		log.Info().Msg("Shutting down Docker Sync")
	},
	PostRun: func(cmd *cobra.Command, args []string) {
		// Wait for a second to allow for any pending log messages to be flushed
		time.Sleep(1 * time.Second)
		if cmd.Annotations["error"] != "" {
			os.Exit(1)
		}
	},
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.dockersync.yaml)")
}

func initConfig() {
	if err := config.InitConfig(cfgFile); err != nil {
		log.Fatal().Err(err).Msg("Failed to initialize config")
	}

	dockersync.Reload()
}
