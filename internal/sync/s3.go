package sync

import (
	"bytes"
	"cmp"
	"context"
	"crypto/md5"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/Altinity/docker-sync/config"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/partial"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/jellydator/ttlcache/v3"
	"github.com/rs/zerolog/log"
	"golang.org/x/sync/errgroup"
)

var bucketInitCache = make(map[string]struct{})

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

func pushS3(ctx context.Context, desc *remote.Descriptor, dst string, repository string, tag string) error {
	s3Session, bucket, err := getS3Session(dst)
	if err != nil {
		return err
	}

	return pushS3WithSession(ctx, s3Session, bucket, dst, repository, desc, tag)
}

func pushS3WithSession(ctx context.Context, s3Session *s3.S3, bucket *string, dst string, repository string, desc *remote.Descriptor, tag string) error {
	s3c := &s3Client{
		uploader:  s3manager.NewUploaderWithClient(s3Session),
		s3Session: s3Session,
		dst:       dst,
		bucket:    bucket,
		acl:       aws.String("public-read"),
		baseDir:   filepath.Join("v2", repository),
	}

	bucketInitCacheKey := fmt.Sprintf("%s/%s", dst, *bucket)
	if _, ok := bucketInitCache[bucketInitCacheKey]; !ok {
		// FIXME: This only needs to be called once per bucket.
		if err := syncObject(
			ctx,
			s3c,
			"v2",
			aws.String("application/json"),
			strings.NewReader("{}"), // We just need to return a 200 and a valid JSON response
		); err != nil {
			return err
		}
		bucketInitCache[bucketInitCacheKey] = struct{}{}
	}

	i, err := desc.Image()
	if err != nil {
		return err
	}

	if err := extractConfigFile(ctx, s3c, i); err != nil {
		return err
	}

	var children []partial.Describable

	idx, err := desc.ImageIndex()
	if err == nil {
		children, err = partial.Manifests(idx)
		if err != nil {
			return err
		}
	}

	var manifests []*manifestWithMediaType
	manifests = append(manifests, &manifestWithMediaType{
		Manifest:  desc.Manifest,
		MediaType: string(desc.MediaType),
		Digest:    desc.Digest.String(),
	})

	var layers []v1.Layer
	l, err := i.Layers()
	if err != nil {
		return err
	}
	layers = append(layers, l...)

	for _, child := range children {
		childManifests, childLayers, err := extractManifestsAndLayers(ctx, s3c, child, manifests, layers)
		if err != nil {
			return err
		}
		manifests = append(manifests, childManifests...)
		layers = append(layers, childLayers...)
	}

	// Deduplicate layers
	slices.SortFunc(layers, func(a, b v1.Layer) int {
		aDigest, _ := a.Digest()
		bDigest, _ := b.Digest()
		return cmp.Compare(aDigest.String(), bDigest.String())
	})
	layers = slices.Compact(layers)

	// Deduplicate manifests
	slices.SortFunc(manifests, func(a, b *manifestWithMediaType) int {
		return cmp.Compare(a.Digest, b.Digest)
	})
	manifests = slices.Compact(manifests)

	log.Info().
		Str("bucket", *bucket).
		Str("repository", repository).
		Str("tag", tag).
		Int("layers", len(layers)).
		Int("manifests", len(manifests)).
		Msg("Syncing objects")

	// Limit upload concurrency
	g, _ := errgroup.WithContext(ctx)
	g.SetLimit(config.SyncS3MaxConcurrentUploads.Int())

	// Layers are synced first to avoid making a tag available before all its blobs are available.
	for _, layer := range layers {
		g.Go(func() error {
			digest, err := layer.Digest()
			if err != nil {
				return err
			}

			key := filepath.Join(s3c.baseDir, "blobs", digest.String())

			mediaType, err := layer.MediaType()
			if err != nil {
				return err
			}

			r, err := layer.Compressed()
			if err != nil {
				return err
			}

			if err := syncObject(
				ctx,
				s3c,
				key,
				aws.String(string(mediaType)),
				r,
			); err != nil {
				return err
			}

			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return err
	}

	// Sync the manifests
	for _, manifest := range manifests {
		g.Go(func() error {
			manifest := manifest

			return syncObject(
				ctx,
				s3c,
				filepath.Join(s3c.baseDir, "manifests", manifest.Digest),
				aws.String(manifest.MediaType),
				bytes.NewReader(manifest.Manifest),
			)
		})
	}

	if err := g.Wait(); err != nil {
		return err
	}

	// Tag is added last so it can be used to check for duplication.
	if err := syncObject(
		ctx,
		s3c,
		manifestKey(repository, tag),
		aws.String(string(desc.MediaType)),
		bytes.NewReader(desc.Manifest),
	); err != nil {
		return err
	}

	return nil
}

func syncObject(
	ctx context.Context,
	s3c *s3Client,
	key string,
	contentType *string,
	r io.Reader,
) error {
	cacheKey := fmt.Sprintf("%s/%s", *s3c.bucket, key)

	if config.SyncS3ObjectCacheEnabled.Bool() {
		if seem := objectCache.Has(cacheKey); seem {
			log.Debug().
				Str("bucket", *s3c.bucket).
				Str("key", key).
				Msg("Object seem recently, skipping upload")
			return nil
		}
	}

	exists, headMetadataDigest, err := s3ObjectExists(s3c.s3Session, s3c.bucket, key)
	if err != nil {
		return err
	}

	fname := path.Base(key)

	// Try to avoid downloading the object if it already exists
	if exists && strings.HasPrefix(fname, "sha256:") && fname == headMetadataDigest {
		log.Debug().
			Str("bucket", *s3c.bucket).
			Str("key", key).
			Msg("Object already exists with same digest, skipping upload")

		if config.SyncS3ObjectCacheEnabled.Bool() {
			objectCache.Set(cacheKey, true, ttlcache.DefaultTTL)
		}
		return nil
	}

	// Blobs can be huge and we need a io.ReadSeeker, so we can't read them all into memory.
	tmpFile, err := os.CreateTemp("", "blob-*")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	sha256Hash := sha256.New()
	md5Hash := md5.New()
	mw := io.MultiWriter(tmpFile, sha256Hash, md5Hash)

	fsize, err := io.Copy(mw, r)
	if err != nil {
		return fmt.Errorf("failed to copy data to temp file: %w", err)
	}

	calculatedDigest := fmt.Sprintf("sha256:%x", sha256Hash.Sum(nil))
	contentMD5 := base64.StdEncoding.EncodeToString(md5Hash.Sum(nil))

	// Try to avoid uploading the object if the hash matches
	if calculatedDigest == headMetadataDigest {
		log.Debug().
			Str("bucket", *s3c.bucket).
			Str("key", key).
			Msg("Object already exists with same digest, skipping upload")

		if config.SyncS3ObjectCacheEnabled.Bool() {
			objectCache.Set(cacheKey, true, ttlcache.DefaultTTL)
		}
		return nil
	}

	// Seek to the start of the file so we can read it again for the S3 upload
	if _, err := tmpFile.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("failed to seek to start of temp file: %w", err)
	}

	log.Info().
		Str("bucket", *s3c.bucket).
		Str("contentType", *contentType).
		Str("computedDigest", calculatedDigest).
		Str("key", key).
		Int64("size", fsize).
		Msg("Uploading object")

	dataCounter := s3DataCounter{
		ctx:  ctx,
		dest: s3c.dst,
		f:    tmpFile,
	}

	if _, err := s3c.uploader.Upload(&s3manager.UploadInput{
		Bucket:      s3c.bucket,
		Key:         aws.String(key),
		Body:        dataCounter,
		ACL:         s3c.acl,
		ContentType: contentType,
		ContentMD5:  aws.String(contentMD5),
		Metadata: map[string]*string{
			"X-Calculated-Digest": aws.String(calculatedDigest),
		},
	}); err != nil {
		return fmt.Errorf("failed to upload object: %w", err)
	}

	if config.SyncS3ObjectCacheEnabled.Bool() {
		objectCache.Set(cacheKey, true, ttlcache.DefaultTTL)
	}

	return nil
}

func s3ObjectExists(s3Session *s3.S3, bucket *string, key string) (bool, string, error) {
	head, err := s3Session.HeadObject(&s3.HeadObjectInput{
		Bucket: bucket,
		Key:    &key,
	})
	if err != nil {
		if awsErr, ok := err.(awserr.Error); ok && awsErr.Code() != "NotFound" {
			return false, "", err
		}
		return false, "", nil
	}

	// R2 only supports MD5, so we need to check the custom X-Calculated-Digest metadata for the SHA256 hash
	var headMetadataDigest string
	if head != nil && head.Metadata != nil {
		headMetadataDigestPtr, digestPresent := head.Metadata["X-Calculated-Digest"]
		if digestPresent {
			headMetadataDigest = *headMetadataDigestPtr
		}
	}

	return true, headMetadataDigest, nil
}
