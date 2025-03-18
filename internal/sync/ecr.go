package sync

import (
	"context"
	"encoding/base64"
	"strings"

	"github.com/Altinity/docker-sync/config"
	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	"github.com/aws/aws-sdk-go-v2/service/ecrpublic"
	"github.com/rs/zerolog/log"
)

func newEcrClient() (*ecr.Client, error) {
	cfg, err := awsconfig.LoadDefaultConfig(context.TODO(),
		awsconfig.WithRegion(config.ECRRegion.String()),
	)
	if err != nil {
		return nil, err
	}

	return ecr.NewFromConfig(cfg), nil
}

func newEcrPublicClient() (*ecrpublic.Client, error) {
	cfg, err := awsconfig.LoadDefaultConfig(context.TODO(),
		awsconfig.WithRegion(config.ECRRegion.String()),
	)
	if err != nil {
		return nil, err
	}

	return ecrpublic.NewFromConfig(cfg), nil
}

func authEcrPrivate(ctx context.Context, repository string) (string, string) {
	client, err := newEcrClient()
	if err != nil {
		log.Error().
			Err(err).
			Msg("Failed to create ECR client, falling back to keychain")

		return "", ""
	}

	out, err := client.GetAuthorizationToken(ctx, nil)
	if err != nil {
		log.Error().
			Err(err).
			Msg("Failed to get ECR authorization token, falling back to keychain")

		return "", ""
	}

	if len(out.AuthorizationData) == 0 {
		log.Error().
			Msg("No authorization data returned from ECR, falling back to keychain")

		return "", ""
	}

	b, err := base64.StdEncoding.DecodeString(*out.AuthorizationData[0].AuthorizationToken)
	if err != nil {
		log.Error().
			Err(err).
			Msg("Failed to decode ECR authorization token, falling back to keychain")

		return "", ""
	}

	parts := strings.SplitN(string(b), ":", 2)
	if len(parts) != 2 {
		log.Error().
			Msg("Invalid ECR authorization token, falling back to keychain")

		return "", ""
	}

	if _, err := client.CreateRepository(ctx, &ecr.CreateRepositoryInput{
		RepositoryName: aws.String(repository),
	}); err != nil && !strings.Contains(err.Error(), "RepositoryAlreadyExistsException") {
		log.Error().
			Err(err).
			Msg("Failed to create ECR repository, pushing might fail")
	}

	return parts[0], parts[1]
}

// FIXME: duplicated code.
func authEcrPublic(ctx context.Context, repository string) (string, string) {
	client, err := newEcrPublicClient()
	if err != nil {
		log.Error().
			Err(err).
			Msg("Failed to create ECR client, falling back to keychain")

		return "", ""
	}

	out, err := client.GetAuthorizationToken(ctx, nil)
	if err != nil {
		log.Error().
			Err(err).
			Msg("Failed to get ECR authorization token, falling back to keychain")

		return "", ""
	}

	b, err := base64.StdEncoding.DecodeString(*out.AuthorizationData.AuthorizationToken)
	if err != nil {
		log.Error().
			Err(err).
			Msg("Failed to decode ECR authorization token, falling back to keychain")

		return "", ""
	}

	parts := strings.SplitN(string(b), ":", 2)
	if len(parts) != 2 {
		log.Error().
			Msg("Invalid ECR authorization token, falling back to keychain")

		return "", ""
	}

	if _, err := client.CreateRepository(ctx, &ecrpublic.CreateRepositoryInput{
		RepositoryName: aws.String(repository),
	}); err != nil && !strings.Contains(err.Error(), "RepositoryAlreadyExistsException") {
		log.Error().
			Err(err).
			Msg("Failed to create ECR repository, pushing might fail")
	}

	return parts[0], parts[1]
}
