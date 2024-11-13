package sync

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
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
		[]byte{}, // No content is needed, we just need to return a 200.
	); err != nil {
		return err
	}

	baseDir := filepath.Join("v2", image.GetName())

	i, err := desc.Image()
	if err != nil {
		return err
	}

	cnf, err := i.RawConfigFile()
	if err != nil {
		return err
	}

	cnfHash, err := i.ConfigName()
	if err != nil {
		return err
	}

	if err := syncObject(
		ctx,
		s3Session,
		bucket,
		filepath.Join(baseDir, "blobs", cnfHash.String()),
		acl,
		aws.String("application/vnd.docker.container.image.v1+json"),
		cnf,
	); err != nil {
		return err
	}

	l, err := i.Layers()
	if err != nil {
		return err
	}

	// Layers are synced first to avoid making a tag available before all its blobs are available.
	for _, layer := range l {
		digest, err := layer.Digest()
		if err != nil {
			return err
		}

		mediaType, err := layer.MediaType()
		if err != nil {
			return err
		}

		var r io.ReadCloser

		if strings.HasSuffix(string(mediaType), ".gzip") {
			r, err = layer.Compressed()
			if err != nil {
				return err
			}
		} else {
			r, err = layer.Uncompressed()
			if err != nil {
				return err
			}
		}

		b, err := io.ReadAll(r)
		if err != nil {
			return err
		}

		if err := syncObject(
			ctx,
			s3Session,
			bucket,
			filepath.Join(baseDir, "blobs", digest.String()),
			acl,
			aws.String(string(mediaType)),
			b,
		); err != nil {
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
		manifest,
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
		manifest,
	); err != nil {
		return err
	}

	return nil
}

func syncObject(ctx context.Context, s3Session *s3.S3, bucket *string, key string, acl *string, contentType *string, b []byte) error {
	head, err := s3Session.HeadObject(&s3.HeadObjectInput{
		Bucket: bucket,
		Key:    &key,
	})
	if err != nil {
		if awsErr, ok := err.(awserr.Error); ok && awsErr.Code() != "NotFound" {
			return err
		}
	}

	if head == nil ||
		head.ContentLength == nil ||
		*head.ContentLength != int64(len(b)) ||
		head.ContentType == nil ||
		*head.ContentType != *contentType {
		log.Info().
			Str("bucket", *bucket).
			Str("key", key).
			Str("contentType", *contentType).
			Int64("contentLength", int64(len(b))).
			Msg("Syncing object")

		if _, err := s3Session.PutObject(&s3.PutObjectInput{
			Bucket:      bucket,
			Key:         &key,
			Body:        bytes.NewReader(b),
			ACL:         acl,
			ContentType: contentType,
		}); err != nil {
			return err
		}
	}

	return nil
}
