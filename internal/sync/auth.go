package sync

import (
	"fmt"

	"github.com/Altinity/docker-sync/config"
	"github.com/Altinity/docker-sync/structs"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/v1/remote"
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

func getSkopeoAuth(url string, name string, side string) ([]string, string) {
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

	if repo.Auth.Token != "" {
		return []string{fmt.Sprintf("--%s-registry-token", side), repo.Auth.Token}, "token"
	}

	if repo.Auth.Username != "" && repo.Auth.Password != "" {
		return []string{fmt.Sprintf("--%s-username", side), repo.Auth.Username, fmt.Sprintf("--%s-password", side), repo.Auth.Password}, "basic"
	}

	switch repo.Auth.Helper {
	case "":
	case "ecr":
		_, basic := authEcrPrivate(name)
		return []string{fmt.Sprintf("--%s-username", side), basic.Username, fmt.Sprintf("--%s-password", side), basic.Password}, "ecr"
	case "ecr-public":
		_, basic := authEcrPublic(name)
		return []string{fmt.Sprintf("--%s-username", side), basic.Username, fmt.Sprintf("--%s-password", side), basic.Password}, "ecr-public"
	default:
		log.Error().
			Str("helper", repo.Auth.Helper).
			Msg("Unknown auth helper, falling back to keychain")
	}

	return nil, "default"
}

func getAuth(url string, name string) (remote.Option, string) {
	repositories := config.SyncRegistries.Repositories()

	var repo *structs.Repository

	for _, r := range repositories {
		if r.URL == url {
			repo = r
			break
		}
	}

	if repo == nil {
		return remote.WithAuthFromKeychain(authn.DefaultKeychain), "default"
	}

	if repo.Auth.Token != "" {
		return remote.WithAuth(&authn.Bearer{
			Token: repo.Auth.Token,
		}), "token"
	}

	if repo.Auth.Username != "" && repo.Auth.Password != "" {
		return remote.WithAuth(&authn.Basic{
			Username: repo.Auth.Username,
			Password: repo.Auth.Password,
		}), "basic"
	}

	switch repo.Auth.Helper {
	case "":
	case "ecr":
		creds, _ := authEcrPrivate(name)
		return creds, "ecr"
	case "ecr-public":
		creds, _ := authEcrPublic(name)
		return creds, "ecr-public"
	default:
		log.Error().
			Str("helper", repo.Auth.Helper).
			Msg("Unknown auth helper, falling back to keychain")
	}

	return remote.WithAuthFromKeychain(authn.DefaultKeychain), "default"
}
