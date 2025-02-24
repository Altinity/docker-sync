package cmd

import (
	"context"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	dockersync "github.com/Altinity/docker-sync"
	"github.com/Altinity/docker-sync/config"
	"github.com/Altinity/docker-sync/logging"
	"github.com/Altinity/docker-sync/structs"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

type syncImage struct {
	Source      string   `yaml:"source"`
	Targets     []string `yaml:"targets"`
	MutableTags []string `yaml:"mutableTags"`
	IgnoredTags []string `yaml:"ignoredTags"`
}

type syncAuth struct {
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	Token    string `yaml:"token"`
	Helper   string `yaml:"helper"`
}

type syncRegistry struct {
	Auth syncAuth `yaml:"auth"`
	Name string   `yaml:"name"`
	URL  string   `yaml:"url"`
}

type syncConfig struct {
	Ecr struct {
		Region string `yaml:"region"`
	} `yaml:"ecr"`
	Sync struct {
		Images     []syncImage    `yaml:"images"`
		Registries []syncRegistry `yaml:"registries"`
	} `yaml:"sync"`
}

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Sync a single image",
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

		cnf := syncConfig{}
		cnf.Ecr.Region, _ = cmd.Flags().GetString("ecr-region")

		source, _ := cmd.Flags().GetString("source")
		target, _ := cmd.Flags().GetString("target")
		mutableTags, _ := cmd.Flags().GetStringSlice("mutableTags")
		ignoredTags, _ := cmd.Flags().GetStringSlice("ignoredTags")

		cnf.Sync.Images = append(cnf.Sync.Images, syncImage{
			Source:      source,
			Targets:     []string{target},
			MutableTags: mutableTags,
			IgnoredTags: ignoredTags,
		})

		var registries []syncRegistry

		sourceHelper, _ := cmd.Flags().GetString("source-helper")
		sourcePassword, _ := cmd.Flags().GetString("source-password")
		sourceToken, _ := cmd.Flags().GetString("source-token")
		sourceUsername, _ := cmd.Flags().GetString("source-username")

		targetHelper, _ := cmd.Flags().GetString("target-helper")
		targetPassword, _ := cmd.Flags().GetString("target-password")
		targetToken, _ := cmd.Flags().GetString("target-token")
		targetUsername, _ := cmd.Flags().GetString("target-username")

		imgHelper := structs.Image{}

		sourceUrl := imgHelper.GetRegistry(source)
		targetUrl := imgHelper.GetRegistry(target)

		if sourceUrl != "" && (sourceUsername != "" || sourcePassword != "" || sourceToken != "" || sourceHelper != "") {
			registries = append(registries, syncRegistry{
				Auth: syncAuth{
					Username: sourceUsername,
					Password: sourcePassword,
					Token:    sourceToken,
					Helper:   sourceHelper,
				},
				Name: "source",
				URL:  sourceUrl,
			})
		}

		if targetUrl != "" && (targetUsername != "" || targetPassword != "" || targetToken != "" || targetHelper != "") {
			registries = append(registries, syncRegistry{
				Auth: syncAuth{
					Username: targetUsername,
					Password: targetPassword,
					Token:    targetToken,
					Helper:   targetHelper,
				},
				Name: "target",
				URL:  targetUrl,
			})
		}

		cnf.Sync.Registries = registries

		tmpDir, err := os.MkdirTemp(os.TempDir(), "docker-sync")
		if err != nil {
			log.Fatal().Err(err).Msg("Failed to create temporary directory")
		}
		defer os.RemoveAll(tmpDir)

		b, err := yaml.Marshal(cnf)
		if err != nil {
			log.Fatal().Err(err).Msg("Failed to marshal configuration")
		}

		if err := os.WriteFile(filepath.Join(tmpDir, "config.yaml"), b, 0644); err != nil {
			log.Fatal().Err(err).Msg("Failed to write configuration")
		}

		if err := config.InitConfig(filepath.Join(tmpDir, "config.yaml")); err != nil {
			log.Fatal().Err(err).Msg("Failed to initialize config")
		}

		dockersync.Reload()

		images := config.SyncImages.Images()

		if err := dockersync.RunOnce(ctx, images); err != nil {
			cmd.Annotations["error"] = err.Error()
			log.Error().
				Stack().
				Err(err).
				Msg("Error running Docker Sync")
		}

		log.Info().Msg("Shutting down Docker Sync")
	},
}

func init() {
	rootCmd.AddCommand(syncCmd)

	syncCmd.Flags().StringP("source", "s", "", "Source image")
	syncCmd.Flags().StringP("target", "t", "", "Target images")

	syncCmd.MarkFlagRequired("source")
	syncCmd.MarkFlagRequired("target")

	syncCmd.Flags().StringSliceP("mutable-tags", "m", []string{}, "Mutable tags")
	syncCmd.Flags().StringSliceP("ignored-tags", "i", []string{}, "Ignored tags")

	syncCmd.Flags().StringP("ecr-region", "", os.Getenv("AWS_REGION"), "AWS region for ECR")

	syncCmd.Flags().StringP("source-helper", "", os.Getenv("DOCKER_SYNC_SOURCE_HELPER"), "Source registry helper")
	syncCmd.Flags().StringP("source-password", "", os.Getenv("DOCKER_SYNC_SOURCE_PASSWORD"), "Source registry password")
	syncCmd.Flags().StringP("source-token", "", os.Getenv("DOCKER_SYNC_SOURCE_TOKEN"), "Source registry token")
	syncCmd.Flags().StringP("source-username", os.Getenv("DOCKER_SYNC_SOURCE_USERNAME"), "", "Source registry username")

	syncCmd.Flags().StringP("target-helper", "", os.Getenv("DOCKER_SYNC_TARGET_HELPER"), "target registry helper")
	syncCmd.Flags().StringP("target-password", "", os.Getenv("DOCKER_SYNC_TARGET_PASSWORD"), "target registry password")
	syncCmd.Flags().StringP("target-token", "", os.Getenv("DOCKER_SYNC_TARGET_TOKEN"), "target registry token")
	syncCmd.Flags().StringP("target-username", os.Getenv("DOCKER_SYNC_TARGET_USERNAME"), "", "target registry username")

	// For more registries and advanced options, please use a configuration file
}
