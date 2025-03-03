package sync

import (
	"encoding/base64"
	"strings"

	"github.com/Altinity/docker-sync/config"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ecr"
	"github.com/aws/aws-sdk-go/service/ecrpublic"
	"github.com/rs/zerolog/log"
)

func newEcrClient() (*ecr.ECR, error) {
	sess, err := session.NewSession(&aws.Config{
		Region: aws.String(config.ECRRegion.String()),
	})
	if err != nil {
		return nil, err
	}

	return ecr.New(sess), nil
}

func newEcrPublicClient() (*ecrpublic.ECRPublic, error) {
	sess, err := session.NewSession(&aws.Config{
		Region: aws.String("us-east-1"),
	})
	if err != nil {
		return nil, err
	}

	return ecrpublic.New(sess), nil
}

func authEcrPrivate(repository string) (string, string) {
	client, err := newEcrClient()
	if err != nil {
		log.Error().
			Err(err).
			Msg("Failed to create ECR client, falling back to keychain")

		return "", ""
	}

	out, err := client.GetAuthorizationToken(nil)
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

	if _, err := client.CreateRepository(&ecr.CreateRepositoryInput{
		RepositoryName: aws.String(repository),
	}); err != nil && !strings.Contains(err.Error(), "RepositoryAlreadyExistsException") {
		log.Error().
			Err(err).
			Msg("Failed to create ECR repository, pushing might fail")
	}

	return parts[0], parts[1]
}

// FIXME: duplicated code.
func authEcrPublic(repository string) (string, string) {
	client, err := newEcrPublicClient()
	if err != nil {
		log.Error().
			Err(err).
			Msg("Failed to create ECR client, falling back to keychain")

		return "", ""
	}

	out, err := client.GetAuthorizationToken(nil)
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

	if _, err := client.CreateRepository(&ecrpublic.CreateRepositoryInput{
		RepositoryName: aws.String(repository),
	}); err != nil && !strings.Contains(err.Error(), "RepositoryAlreadyExistsException") {
		log.Error().
			Err(err).
			Msg("Failed to create ECR repository, pushing might fail")
	}

	return parts[0], parts[1]
}
