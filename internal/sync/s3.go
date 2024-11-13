package sync

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Altinity/docker-sync/structs"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/rs/zerolog/log"
)

func getS3Session(url string) (*s3.S3, *string, error) {
	fields := strings.Split(url, ":")
	if len(fields) != 4 {
		return nil, nil, fmt.Errorf("invalid S3 destination: %s, format is s3:<region>:<bucket>:<image>", url)
	}

	accessKey, secretKey, err := getObjectStorageAuth(strings.Join(fields[:3], ":"))
	if err != nil {
		return nil, nil, err
	}

	region := aws.String(fields[1])
	bucket := aws.String(fields[2])

	newSession, err := session.NewSession(&aws.Config{
		Credentials:      credentials.NewStaticCredentials(accessKey, secretKey, ""),
		Region:           region,
		S3ForcePathStyle: aws.Bool(true),
		HTTPClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	})
	if err != nil {
		return nil, nil, err
	}

	return s3.New(newSession), bucket, nil
}

func pushS3(ctx context.Context, image *structs.Image, desc *remote.Descriptor, dst string, tag string) error {
	s3Session, bucket, err := getS3Session(dst)
	if err != nil {
		return err
	}

	return pushS3WithSession(ctx, s3Session, bucket, image, desc, dst, tag)
}

func pushS3WithSession(ctx context.Context, s3Session *s3.S3, bucket *string, image *structs.Image, desc *remote.Descriptor, dst string, tag string) error {
	acl := aws.String("public-read")

	// FIXME: this only needs to be called once per bucket
	if err := syncObject(
		ctx,
		s3Session,
		bucket,
		"v2",
		acl,
		aws.String("application/json"),
		bytes.NewReader([]byte{}), // No content is needed, we just need to return a 200.
	); err != nil {
		return err
	}

	baseDir := filepath.Join("v2", image.GetSourceRepository())

	i, err := desc.Image()
	if err != nil {
		return err
	}

	cnf, err := i.RawConfigFile()
	// Config is optional, so we don't return an error if it's not found.
	if err == nil {
		cnfHash, err := i.ConfigName()
		if err == nil {
			if err := syncObject(
				ctx,
				s3Session,
				bucket,
				filepath.Join(baseDir, "blobs", cnfHash.String()),
				acl,
				aws.String("application/vnd.docker.container.image.v1+json"),
				bytes.NewReader(cnf),
			); err != nil {
				return err
			}
		}
	}

	l, err := i.Layers()
	if err != nil {
		return err
	}

	// Blobs can be huge and we need a io.ReadSeeker, so we can't read them all into memory.
	tmpDir, err := os.MkdirTemp(os.TempDir(), "docker-sync")
	if err != nil {
		return err
	}

	// Layers are synced first to avoid making a tag available before all its blobs are available.
	for _, layer := range l {
		if err := func() error {
			digest, err := layer.Digest()
			if err != nil {
				return err
			}

			mediaType, err := layer.MediaType()
			if err != nil {
				return err
			}

			r, err := layer.Compressed()
			if err != nil {
				return err
			}

			tmpFile, err := os.Create(filepath.Join(tmpDir, "blob"))
			if err != nil {
				return err
			}
			defer os.Remove(tmpFile.Name())
			defer tmpFile.Close()

			if _, err := io.Copy(tmpFile, r); err != nil {
				return err
			}
			tmpFile.Seek(0, io.SeekStart)

			if err := syncObject(
				ctx,
				s3Session,
				bucket,
				filepath.Join(baseDir, "blobs", digest.String()),
				acl,
				aws.String(string(mediaType)),
				tmpFile,
			); err != nil {
				return err
			}

			return nil
		}(); err != nil {
			return err
		}
	}

	mediaType := aws.String(string(desc.MediaType))

	manifest := desc.Manifest

	if err := syncObject(
		ctx,
		s3Session,
		bucket,
		filepath.Join(baseDir, "manifests", tag),
		acl,
		mediaType,
		bytes.NewReader(manifest),
	); err != nil {
		return err
	}

	if err := syncObject(
		ctx,
		s3Session,
		bucket,
		filepath.Join(baseDir, "manifests", desc.Digest.String()),
		acl,
		mediaType,
		bytes.NewReader(manifest),
	); err != nil {
		return err
	}

	return nil
}

func syncObject(ctx context.Context, s3Session *s3.S3, bucket *string, key string, acl *string, contentType *string, r io.ReadSeeker) error {
	h := sha256.New()
	if _, err := io.Copy(h, r); err != nil {
		return err
	}
	calculatedDigest := fmt.Sprintf("sha256:%x", h.Sum(nil))
	r.Seek(0, io.SeekStart)

	head, err := s3Session.HeadObject(&s3.HeadObjectInput{
		Bucket: bucket,
		Key:    &key,
	})
	if err != nil {
		if awsErr, ok := err.(awserr.Error); ok && awsErr.Code() != "NotFound" {
			return err
		}
	}

	headMetadataDigest, digestPresent := head.Metadata["calculatedDigest"]

	if head == nil ||
		head.ContentType == nil ||
		*head.ContentType != *contentType ||
		!digestPresent ||
		*headMetadataDigest != calculatedDigest {
		log.Info().
			Str("bucket", *bucket).
			Str("key", key).
			Str("contentType", *contentType).
			Msg("Syncing object")

		if _, err := s3Session.PutObject(&s3.PutObjectInput{
			Bucket:      bucket,
			Key:         &key,
			Body:        r,
			ACL:         acl,
			ContentType: contentType,
			Metadata: map[string]*string{
				"calculatedDigest": aws.String(calculatedDigest),
			},
		}); err != nil {
			return err
		}
	}

	return nil
}
