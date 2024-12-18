package sync

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/Altinity/docker-sync/structs"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

func listS3Tags(dst string, fields []string) ([]string, error) {
	var s3Session *s3.S3
	var bucket *string
	var err error

	switch fields[0] {
	case "r2":
		s3Session, bucket, err = getR2Session(dst)
	case "s3":
		s3Session, bucket, err = getS3Session(dst)
	default:
		return nil, fmt.Errorf("unsupported bucket destination: %s", dst)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to get bucket session: %w", err)
	}

	s3Lister, err := s3Session.ListObjectsV2(&s3.ListObjectsV2Input{
		Bucket: bucket,
		Prefix: aws.String(filepath.Join("v2", fields[3], "manifests")),
	})
	if err != nil {
		return nil, err
	}

	var tags []string

	for _, obj := range s3Lister.Contents {
		fname := filepath.Base(*obj.Key)
		if !strings.HasPrefix(fname, "sha256:") {
			tags = append(tags, fmt.Sprintf("%s:%s", dst, fname))
		}
	}

	return tags, nil
}

func listOCITags(ctx context.Context, auth remote.Option, image *structs.Image, includeRepo string) ([]string, error) {
	puller, err := remote.NewPuller(auth)
	if err != nil {
		return nil, err
	}

	repo, err := name.NewRepository(image.Source)
	if err != nil {
		return nil, err
	}

	lister, err := puller.Lister(ctx, repo)
	if err != nil {
		return nil, err
	}

	var allTags []string

	for lister.HasNext() {
		tags, err := lister.Next(ctx)
		if err != nil {
			return nil, err
		}

		for _, tag := range tags.Tags {
			if includeRepo == "" {
				allTags = append(allTags, tag)
			} else {
				allTags = append(allTags, fmt.Sprintf("%s:%s", includeRepo, tag))
			}
		}
	}

	return allTags, nil
}
