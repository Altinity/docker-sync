package sync

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/Masterminds/semver"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

func isSemVerTag(tag string) bool {
	_, err := semver.NewVersion(tag)
	return err == nil
}

func listS3Tags(ctx context.Context, dst string, fields []string) ([]string, error) {
	var s3Session *s3.Client
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

	p := s3.NewListObjectsV2Paginator(s3Session, &s3.ListObjectsV2Input{
		Bucket: bucket,
		Prefix: aws.String(filepath.Join("v2", fields[3], "manifests")),
	})

	var tags []string

	var i int
	for p.HasMorePages() {
		i++
		page, err := p.NextPage(ctx)
		if err != nil {
			return tags, fmt.Errorf("failed to get page %d, %w", i, err)
		}
		for _, obj := range page.Contents {
			fname := filepath.Base(*obj.Key)
			if !strings.HasPrefix(fname, "sha256:") {
				tags = append(tags, fmt.Sprintf("%s:%s", dst, fname))
			}
		}
	}

	return tags, nil
}
