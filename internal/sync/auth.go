package sync

import (
	"fmt"

	"github.com/Altinity/docker-sync/config"
	"github.com/Altinity/docker-sync/structs"
	"github.com/containers/image/v5/types"
	"github.com/rs/zerolog/log"
)

func getObjectStorageAuth(url string) (string, string, error) {
	repositories := config.SyncRegistries.Repositories()

	var repo *structs.Repository

	for _, r := range repositories {
		if r.URL == url {
			repo = r
			break
		}
	}

	if repo.Auth.Username != "" && repo.Auth.Password != "" {
		return repo.Auth.Username, repo.Auth.Password, nil
	}

	return "", "", fmt.Errorf("no auth found for %s", url)
}

func getSkopeoAuth(url string, name string) (*types.DockerAuthConfig, string) {
	repositories := config.SyncRegistries.Repositories()

	var repo *structs.Repository

	for _, r := range repositories {
		if r.URL == url {
			repo = r
			break
		}
	}

	if repo == nil {
		return nil, "default"
	}

	if repo.Auth.Username != "" && repo.Auth.Password != "" {
		return &types.DockerAuthConfig{Username: repo.Auth.Username, Password: repo.Auth.Password}, "basic"
	}

	switch repo.Auth.Helper {
	case "":
	case "ecr":
		username, password := authEcrPrivate(name)
		return &types.DockerAuthConfig{Username: username, Password: password}, "ecr"
	case "ecr-public":
		username, password := authEcrPublic(name)
		return &types.DockerAuthConfig{Username: username, Password: password}, "ecr-public"
	default:
		log.Error().
			Str("helper", repo.Auth.Helper).
			Msg("Unknown auth helper, falling back to keychain")
	}

	return nil, "default"
}
