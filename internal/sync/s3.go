package sync

import (
	"bytes"
	"crypto/md5"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
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
			Timeout: 300 * time.Second, // Some blobs are huge
		},
	})
	if err != nil {
		return nil, nil, err
	}

	return s3.New(newSession), bucket, nil
}

func pushS3(image *structs.Image, desc *remote.Descriptor, dst string, tag string) error {
	s3Session, bucket, err := getS3Session(dst)
	if err != nil {
		return err
	}

	return pushS3WithSession(s3Session, bucket, image, desc, tag)
}

func pushS3WithSession(s3Session *s3.S3, bucket *string, image *structs.Image, desc *remote.Descriptor, tag string) error {
	acl := aws.String("public-read")

	// FIXME: this only needs to be called once per bucket
	if err := syncObject(
		s3Session,
		bucket,
		"v2",
		acl,
		aws.String("application/json"),
		bytes.NewReader([]byte("{}")), // No content is needed, we just need to return a 200.
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

			key := filepath.Join(baseDir, "blobs", digest.String())

			exists, _, headMetadataDigest, err := s3ObjectExists(s3Session, bucket, key)
			if err != nil {
				return err
			} else if exists && fmt.Sprintf("sha256:%s", headMetadataDigest) == digest.String() {
				return nil
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

			if err := syncObject(
				s3Session,
				bucket,
				key,
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
		s3Session,
		bucket,
		filepath.Join(baseDir, "manifests", desc.Digest.String()),
		acl,
		mediaType,
		bytes.NewReader(manifest),
	); err != nil {
		return err
	}

	// Tag is added last so it can be used to check for duplication.
	if err := syncObject(
		s3Session,
		bucket,
		manifestKey(image, tag),
		acl,
		mediaType,
		bytes.NewReader(manifest),
	); err != nil {
		return err
	}

	return nil
}

func syncObject(s3Session *s3.S3, bucket *string, key string, acl *string, contentType *string, r io.ReadSeeker) error {
	_, _, headMetadataDigest, err := s3ObjectExists(s3Session, bucket, key)
	if err != nil {
		return err
	}

	var calculatedDigest string

	// If we are uploading a blob, trust it's digest
	fname := path.Base(key)
	if strings.HasPrefix(fname, "sha256:") {
		fields := strings.Split(fname, ":")
		if len(fields) == 2 {
			calculatedDigest = fields[1]
		}
	}

	if calculatedDigest == "" {
		r.Seek(0, io.SeekStart)
		h := md5.New()
		if _, err := io.Copy(h, r); err != nil {
			return err
		}
		calculatedDigest = fmt.Sprintf("%x", h.Sum(nil))
		r.Seek(0, io.SeekStart)
	}

	if calculatedDigest != headMetadataDigest {
		log.Info().
			Str("bucket", *bucket).
			Str("key", key).
			Str("contentType", *contentType).
			Msg("Syncing object")

		if _, err := s3Session.PutObject(&s3.PutObjectInput{
			Bucket:      bucket,
			Key:         aws.String(key),
			Body:        r,
			ACL:         acl,
			ContentType: contentType,
			Metadata: map[string]*string{
				"X-Calculated-Digest": aws.String(calculatedDigest),
			},
		}); err != nil {
			return err
		}
	}

	return nil
}

func manifestKey(image *structs.Image, tag string) string {
	return filepath.Join("v2", image.GetSourceRepository(), "manifests", tag)
}

func s3ObjectExists(s3Session *s3.S3, bucket *string, key string) (bool, string, string, error) {
	head, err := s3Session.HeadObject(&s3.HeadObjectInput{
		Bucket: bucket,
		Key:    &key,
	})
	if err != nil {
		if awsErr, ok := err.(awserr.Error); ok && awsErr.Code() != "NotFound" {
			return false, "", "", err
		}
		return false, "", "", nil
	}

	var etag string
	if head != nil && head.ETag != nil {
		etag = strings.ReplaceAll(*head.ETag, `"`, "")
	}

	// R2 only supports MD5, so we need to check the custom X-Calculated-Digest metadata for the SHA256 hash
	var headMetadataDigest string
	if head != nil && head.Metadata != nil {
		headMetadataDigestPtr, digestPresent := head.Metadata["X-Calculated-Digest"]
		if digestPresent {
			headMetadataDigest = *headMetadataDigestPtr
		}
	}

	return true, etag, headMetadataDigest, nil
}
