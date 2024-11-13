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
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

type syncImage struct {
	Source      string   `yaml:"source"`
	Targets     []string `yaml:"targets"`
	MutableTags []string `yaml:"mutableTags"`
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
		targets, _ := cmd.Flags().GetStringSlice("targets")

		cnf.Sync.Images = append(cnf.Sync.Images, syncImage{
			Source:  source,
			Targets: targets,
		})

		var registries []syncRegistry

		helper1, _ := cmd.Flags().GetString("helper1")
		password1, _ := cmd.Flags().GetString("password1")
		token1, _ := cmd.Flags().GetString("token1")
		username1, _ := cmd.Flags().GetString("username1")
		url1, _ := cmd.Flags().GetString("url1")

		helper2, _ := cmd.Flags().GetString("helper2")
		password2, _ := cmd.Flags().GetString("password2")
		token2, _ := cmd.Flags().GetString("token2")
		username2, _ := cmd.Flags().GetString("username2")
		url2, _ := cmd.Flags().GetString("url2")

		helper3, _ := cmd.Flags().GetString("helper3")
		password3, _ := cmd.Flags().GetString("password3")
		token3, _ := cmd.Flags().GetString("token3")
		username3, _ := cmd.Flags().GetString("username3")
		url3, _ := cmd.Flags().GetString("url3")

		if url1 != "" && (username1 != "" || password1 != "" || token1 != "" || helper1 != "") {
			registries = append(registries, syncRegistry{
				Auth: syncAuth{
					Username: username1,
					Password: password1,
					Token:    token1,
					Helper:   helper1,
				},
				Name: "registry1",
				URL:  url1,
			})
		}

		if url2 != "" && (username2 != "" || password2 != "" || token2 != "" || helper2 != "") {
			registries = append(registries, syncRegistry{
				Auth: syncAuth{
					Username: username2,
					Password: password2,
					Token:    token2,
					Helper:   helper2,
				},
				Name: "registry2",
				URL:  url2,
			})
		}

		if url3 != "" && (username3 != "" || password3 != "" || token3 != "" || helper3 != "") {
			registries = append(registries, syncRegistry{
				Auth: syncAuth{
					Username: username3,
					Password: password3,
					Token:    token3,
					Helper:   helper3,
				},
				Name: "registry3",
				URL:  url3,
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
	syncCmd.Flags().StringSliceP("targets", "t", []string{}, "Target images")

	syncCmd.MarkFlagRequired("source")
	syncCmd.MarkFlagRequired("targets")

	syncCmd.Flags().StringP("mutableTags", "m", "", "Mutable tags")

	syncCmd.Flags().StringP("ecr-region", "", os.Getenv("AWS_REGION"), "AWS region for ECR")

	syncCmd.Flags().StringP("helper1", "", os.Getenv("DOCKER_SYNC_REGISTRY_1_HELPER"), "First registry helper for target image")
	syncCmd.Flags().StringP("password1", "", os.Getenv("DOCKER_SYNC_REGISTRY_1_PASSWORD"), "First registry password for target image")
	syncCmd.Flags().StringP("token1", "", os.Getenv("DOCKER_SYNC_REGISTRY_1_TOKEN"), "First registry token for target image")
	syncCmd.Flags().StringP("username1", os.Getenv("DOCKER_SYNC_TARGET_REGISTRY_1_USERNAME"), "", "First registry username for target image")
	syncCmd.Flags().StringP("url1", "", os.Getenv("DOCKER_SYNC_TARGET_REGISTRY_1_URL"), "First registry URL for target image")

	syncCmd.Flags().StringP("helper2", "", os.Getenv("DOCKER_SYNC_REGISTRY_2_HELPER"), "Second registry helper for target image")
	syncCmd.Flags().StringP("password2", "", os.Getenv("DOCKER_SYNC_REGISTRY_2_PASSWORD"), "Second registry password for target image")
	syncCmd.Flags().StringP("token2", "", os.Getenv("DOCKER_SYNC_REGISTRY_2_TOKEN"), "Second registry token for target image")
	syncCmd.Flags().StringP("username2", os.Getenv("DOCKER_SYNC_TARGET_REGISTRY_2_USERNAME"), "", "Second registry username for target image")
	syncCmd.Flags().StringP("url2", "", os.Getenv("DOCKER_SYNC_TARGET_REGISTRY_2_URL"), "Second registry URL for target image")

	syncCmd.Flags().StringP("helper3", "", os.Getenv("DOCKER_SYNC_REGISTRY_3_HELPER"), "Third registry helper for target image")
	syncCmd.Flags().StringP("password3", "", os.Getenv("DOCKER_SYNC_REGISTRY_3_PASSWORD"), "Third registry password for target image")
	syncCmd.Flags().StringP("token3", "", os.Getenv("DOCKER_SYNC_REGISTRY_3_TOKEN"), "Third registry token for target image")
	syncCmd.Flags().StringP("username3", os.Getenv("DOCKER_SYNC_TARGET_REGISTRY_3_USERNAME"), "", "Third registry username for target image")
	syncCmd.Flags().StringP("url3", "", os.Getenv("DOCKER_SYNC_TARGET_REGISTRY_3_URL"), "Third registry URL for target image")

	// For more registries, please use a configuration file
}
