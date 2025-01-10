package sync

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Altinity/docker-sync/structs"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
)

func getR2Session(url string) (*s3.S3, *string, error) {
	fields := strings.Split(url, ":")
	if len(fields) != 4 {
		return nil, nil, fmt.Errorf("invalid R2 destination: %s, format is r2:<endpoint>:<bucket>:<image>", url)
	}

	accessKey, secretKey, err := getObjectStorageAuth(strings.Join(fields[:3], ":"))
	if err != nil {
		return nil, nil, err
	}

	endpoint := aws.String(fmt.Sprintf("https://%s.r2.cloudflarestorage.com", fields[1]))
	bucket := aws.String(fields[2])

	newSession, err := session.NewSession(&aws.Config{
		Credentials:      credentials.NewStaticCredentials(accessKey, secretKey, ""),
		Endpoint:         endpoint,
		Region:           aws.String("us-east-1"),
		S3ForcePathStyle: aws.Bool(true),
		HTTPClient: &http.Client{
			Timeout: 300 * time.Second, // Some blobs are huge
		},
	})
	if err != nil {
		return nil, nil, err
	}
	return s3.New(newSession), bucket, nil
}

func pushR2(ctx context.Context, image *structs.Image, dst string, repository string, tag string) error {
	s3Session, bucket, err := getR2Session(dst)
	if err != nil {
		return err
	}

	return pushS3WithSession(ctx, s3Session, bucket, dst, repository, image, tag)
}
