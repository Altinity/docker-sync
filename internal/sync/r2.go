package sync

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Altinity/docker-sync/structs"
	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

func getR2Session(url string) (*s3.Client, *string, error) {
	fields := strings.Split(url, ":")
	if len(fields) != 4 {
		return nil, nil, fmt.Errorf("invalid R2 destination: %s, format is r2:<endpoint>:<bucket>:<image>", url)
	}

	if fields[0] != "r2" {
		return nil, nil, fmt.Errorf("invalid protocol: %s, expected r2", fields[0])
	}

	accessKey, secretKey, err := getObjectStorageAuth(strings.Join(fields[:3], ":"))
	if err != nil {
		return nil, nil, err
	}

	endpoint := aws.String(fmt.Sprintf("https://%s.r2.cloudflarestorage.com", fields[1]))
	bucket := aws.String(fields[2])

	cfg, err := awsconfig.LoadDefaultConfig(context.TODO(),
		awsconfig.WithRegion("us-east-1"),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKey, secretKey, "")),
		awsconfig.WithHTTPClient(&http.Client{Timeout: 300 * time.Second}),
	)
	if err != nil {
		return nil, nil, err
	}
	return s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = endpoint
	}), bucket, nil
}

func pushR2(ctx context.Context, image *structs.Image, dst string, repository string, tag string) error {
	s3Session, bucket, err := getR2Session(dst)
	if err != nil {
		return err
	}

	return pushS3WithSession(ctx, s3Session, bucket, dst, repository, image, tag)
}

func deleteR2(ctx context.Context, image *structs.Image, dst string, repository string, tag string) error {
	s3Session, bucket, err := getR2Session(dst)
	if err != nil {
		return err
	}

	return deleteS3WithSession(ctx, s3Session, bucket, dst, repository, tag)
}
