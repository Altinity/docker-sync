package sync

import (
	"bytes"
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"

	"github.com/Altinity/docker-sync/config"
	"github.com/Altinity/docker-sync/structs"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/rs/zerolog/log"
	"golang.org/x/sync/errgroup"
)

func deleteS3(ctx context.Context, image *structs.Image, dst string, repository string, tag string) error {
	s3Session, bucket, err := getS3Session(dst)
	if err != nil {
		return err
	}

	return deleteS3WithSession(ctx, s3Session, bucket, dst, repository, tag)
}

func deleteS3WithSession(ctx context.Context, s3Session *s3.Client, bucket *string, dst string, repository string, tag string) error {
	s3c := &s3Client{
		s3Session: s3Session,
		bucket:    bucket,
		dst:       dst,
		baseDir:   filepath.Join("v2", repository),
	}

	if err := deleteObject(ctx, s3c, filepath.Join(s3c.baseDir, "manifests", tag)); err != nil {
		return err
	}

	return nil
}

func deleteObject(ctx context.Context, s3c *s3Client, key string) error {
	log.Info().
		Str("bucket", *s3c.bucket).
		Str("key", key).
		Msg("Deleting object")

	_, err := s3c.s3Session.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(*s3c.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("failed to delete object %s from bucket %s: %w", key, *s3c.bucket, err)
	}

	if config.SyncS3ObjectCacheEnabled.Bool() {
		cacheKey := fmt.Sprintf("%s/%s", *s3c.bucket, key)
		objectCache.Delete(cacheKey)
	}

	return nil
}

func getAllRepositoryBlobsS3(ctx context.Context, s3Session *s3.Client, bucket string, repository string) ([]string, error) {
	log.Info().
		Str("bucket", bucket).
		Str("repository", repository).
		Msg("Getting all blobs in repository")

	blobs := []string{}

	p := s3.NewListObjectsV2Paginator(s3Session, &s3.ListObjectsV2Input{
		Bucket: aws.String(bucket),
		Prefix: aws.String(filepath.Join("v2", repository, "blobs")),
	})

	var i int
	for p.HasMorePages() {
		i++
		page, err := p.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get page %d, %w", i, err)
		}
		for _, obj := range page.Contents {
			fname := filepath.Base(*obj.Key)
			if strings.HasPrefix(fname, "sha256:") {
				blobs = append(blobs, fname)
			}
		}
	}

	slices.Sort(blobs)

	return slices.Compact(blobs), nil
}

func getAllReferencedBlobsS3(ctx context.Context, s3Session *s3.Client, bucket string, repository string) ([]string, error) {
	log.Info().
		Str("bucket", bucket).
		Str("repository", repository).
		Msg("Getting all referenced blobs")

	blobs := []string{}

	p := s3.NewListObjectsV2Paginator(s3Session, &s3.ListObjectsV2Input{
		Bucket: aws.String(bucket),
		Prefix: aws.String(filepath.Join("v2", repository, "manifests")),
	})

	var blobsMutex sync.Mutex

	var i int
	for p.HasMorePages() {
		i++
		page, err := p.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get page %d, %w", i, err)
		}

		log.Debug().
			Str("bucket", bucket).
			Int("page", i).
			Int("objects", len(page.Contents)).
			Msg("Processing page of objects")

		g, _ := errgroup.WithContext(ctx)
		g.SetLimit(config.SyncS3MaxPurgeConcurrency.Int())

		for _, obj := range page.Contents {
			g.Go(func() error {
				log.Debug().
					Str("bucket", bucket).
					Str("key", *obj.Key).
					Msg("Processing object")

				// We need to read the manifest to find out which blobs it references
				resp, err := s3Session.GetObject(ctx, &s3.GetObjectInput{
					Bucket: aws.String(bucket),
					Key:    obj.Key,
				})
				if err != nil {
					log.Error().
						Err(err).
						Str("bucket", bucket).
						Str("key", *obj.Key).
						Msg("Failed to get object")

					return fmt.Errorf("failed to get object %s from bucket %s: %w", *obj.Key, bucket, err)
				}
				defer resp.Body.Close()

				// To avoid having to parse the manifest, we can just read the body and look for "sha256:<64 chars>" patterns using regex
				buf := new(bytes.Buffer)
				if _, err := buf.ReadFrom(resp.Body); err != nil {
					// Don't proceed because we can miss blobs
					return fmt.Errorf("failed to read object body: %w", err)
				}

				body := buf.String()
				// Find all sha256 hashes in the body
				re := regexp.MustCompile(`sha256:[a-f0-9]{64}`)
				matches := re.FindAllString(body, -1)

				blobsMutex.Lock()
				for _, match := range matches {
					blobs = append(blobs, match)
				}
				blobsMutex.Unlock()

				return nil
			})
		}

		if err := g.Wait(); err != nil {
			return nil, fmt.Errorf("failed to process page %d: %w", i, err)
		}
	}

	slices.Sort(blobs)

	return slices.Compact(blobs), nil
}

func deleteOrphanedBlobsS3(ctx context.Context, s3Session *s3.Client, bucket string, repository string) error {
	// Get all blobs in the repository
	allBlobs, err := getAllRepositoryBlobsS3(ctx, s3Session, bucket, repository)
	if err != nil {
		return fmt.Errorf("failed to get all blobs: %w", err)
	}
	log.Info().
		Str("bucket", bucket).
		Str("repository", repository).
		Int("blobs", len(allBlobs)).
		Msg("Retrieved all blobs in repository")

	// Get all referenced blobs in the repository
	referencedBlobs, err := getAllReferencedBlobsS3(ctx, s3Session, bucket, repository)
	if err != nil {
		return fmt.Errorf("failed to get all referenced blobs: %w", err)
	}
	log.Info().
		Str("bucket", bucket).
		Str("repository", repository).
		Int("referenced_blobs", len(referencedBlobs)).
		Msg("Retrieved all referenced blobs in repository")

	// Find orphaned blobs
	var orphanedBlobs []string
	for _, blob := range allBlobs {
		if !slices.Contains(referencedBlobs, blob) {
			orphanedBlobs = append(orphanedBlobs, blob)
		}
	}

	if len(orphanedBlobs) == 0 {
		log.Info().
			Str("bucket", bucket).
			Str("repository", repository).
			Msg("No orphaned blobs found")
		return nil
	}

	log.Info().
		Str("bucket", bucket).
		Str("repository", repository).
		Int("orphaned_blobs", len(orphanedBlobs)).
		Msg("Found orphaned blobs")

	g, _ := errgroup.WithContext(ctx)
	g.SetLimit(config.SyncS3MaxPurgeConcurrency.Int())

	for _, blob := range orphanedBlobs {
		g.Go(func() error {
			key := filepath.Join("v2", repository, "blobs", blob)
			if err := deleteObject(ctx, &s3Client{
				s3Session: s3Session,
				bucket:    aws.String(bucket),
			}, key); err != nil {
				log.Error().
					Err(err).
					Str("bucket", bucket).
					Str("key", key).
					Msg("Failed to delete orphaned blob")

				return err
			}

			return nil
		})
	}

	_ = g.Wait()

	return nil
}
