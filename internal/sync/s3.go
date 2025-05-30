package sync

import (
	"bytes"
	"context"
	"crypto/md5"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	awstypes "github.com/aws/aws-sdk-go-v2/service/s3/types"

	"github.com/containers/image/v5/copy"
	"github.com/containers/image/v5/docker"
	"github.com/containers/image/v5/signature"

	"github.com/Altinity/docker-sync/config"
	"github.com/Altinity/docker-sync/structs"
	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/containers/image/v5/directory"
	"github.com/containers/image/v5/types"
	"github.com/jellydator/ttlcache/v3"
	"github.com/rs/zerolog/log"
	"golang.org/x/sync/errgroup"
)

var bucketInitCache = make(map[string]struct{})

func getS3Session(url string) (*s3.Client, *string, error) {
	fields := strings.Split(url, ":")
	if len(fields) != 4 {
		return nil, nil, fmt.Errorf("invalid S3 destination: %s, format is s3:<region>:<bucket>:<image>", url)
	}

	accessKey, secretKey, err := getObjectStorageAuth(strings.Join(fields[:3], ":"))
	if err != nil {
		return nil, nil, err
	}

	region := fields[1]
	bucket := aws.String(fields[2])

	cfg, err := awsconfig.LoadDefaultConfig(context.TODO(),
		awsconfig.WithRegion(region),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKey, secretKey, "")),
		awsconfig.WithHTTPClient(&http.Client{Timeout: 300 * time.Second}),
	)
	if err != nil {
		return nil, nil, err
	}

	return s3.NewFromConfig(cfg), bucket, nil
}

func pushS3(ctx context.Context, image *structs.Image, dst string, repository string, tag string) error {
	s3Session, bucket, err := getS3Session(dst)
	if err != nil {
		return err
	}

	return pushS3WithSession(ctx, s3Session, bucket, dst, repository, image, tag)
}

func pushS3WithSession(ctx context.Context, s3Session *s3.Client, bucket *string, dst string, repository string, image *structs.Image, tag string) error {
	s3c := &s3Client{
		uploader:  manager.NewUploader(s3Session),
		s3Session: s3Session,
		dst:       dst,
		bucket:    bucket,
		acl:       aws.String("public-read"),
		baseDir:   filepath.Join("v2", repository),
	}

	bucketInitCacheKey := fmt.Sprintf("%s/%s", image.GetRegistry(dst), *bucket)
	if _, ok := bucketInitCache[bucketInitCacheKey]; !ok {
		if err := syncObject(
			ctx,
			s3c,
			"v2",
			aws.String("application/json"),
			strings.NewReader("{}"), // We just need to return a 200 and a valid JSON response
			false,
		); err != nil {
			return err
		}
		bucketInitCache[bucketInitCacheKey] = struct{}{}
	}

	tmpDir, err := os.MkdirTemp(os.TempDir(), "docker-sync-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	baseDir, err := os.MkdirTemp(os.TempDir(), "docker-sync-base-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(baseDir)

	baseDir = filepath.Join(baseDir, "v2", repository)

	if err := os.MkdirAll(filepath.Join(baseDir, "blobs"), 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(baseDir, "manifests"), 0o755); err != nil {
		return err
	}

	dstRef, err := directory.NewReference(tmpDir)
	if err != nil {
		return err
	}

	srcRef, err := docker.ParseReference(fmt.Sprintf("//%s:%s", image.Source, tag))
	if err != nil {
		return err
	}

	srcAuth, _ := getSkopeoAuth(ctx, image.GetSourceRegistry(), image.GetSourceRepository())
	srcCtx := &types.SystemContext{
		DockerAuthConfig: srcAuth,
	}

	policy := &signature.Policy{Default: []signature.PolicyRequirement{signature.NewPRInsecureAcceptAnything()}}
	policyContext, err := signature.NewPolicyContext(policy)
	if err != nil {
		return err
	}

	ch := make(chan types.ProgressProperties)
	defer close(ch)

	chCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	go dockerDataCounter(chCtx, image.Source, "", ch)

	if _, err := copy.Image(ctx, policyContext, dstRef, srcRef, &copy.Options{
		SourceCtx:          srcCtx,
		ImageListSelection: copy.CopyAllImages,
		ProgressInterval:   time.Second,
		Progress:           ch,
	}); err != nil {
		return err
	}

	var blobs []string
	var manifests []string
	var tagManifest string

	// walk all files
	if err := filepath.WalkDir(tmpDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		fi, err := os.Stat(path)
		if err != nil {
			return err
		}
		if fi.IsDir() {
			return nil
		}

		base := filepath.Base(path)
		switch {
		case base == "version":
			os.Remove(path)
		case base == "manifest.json":
			f, err := os.Open(path)
			if err != nil {
				return err
			}
			if err := func() error {
				defer f.Close()

				newPath := filepath.Join(baseDir, "manifests", tag)

				if err := os.Link(path, newPath); err != nil {
					return err
				}

				tagManifest = newPath

				return nil
			}(); err != nil {
				return err
			}

			newPath, err := shamove(baseDir, path, "manifests")
			if err != nil {
				return err
			}

			manifests = append(manifests, newPath)
		case strings.HasSuffix(path, ".manifest.json"):
			newPath, err := shamove(baseDir, path, "manifests")
			if err != nil {
				return err
			}
			manifests = append(manifests, newPath)
		default:
			newPath, err := shamove(baseDir, path, "blobs")
			if err != nil {
				return err
			}
			blobs = append(blobs, newPath)
		}

		return nil
	}); err != nil {
		return err
	}

	// Adds the tag manifest last to all others are uploaded first
	manifests = append(manifests, tagManifest)

	log.Info().
		Str("bucket", *bucket).
		Str("repository", repository).
		Str("tag", tag).
		Int("layers", len(blobs)).
		Int("manifests", len(manifests)).
		Msg("Syncing objects")

	// Limit upload concurrency
	g, _ := errgroup.WithContext(ctx)
	g.SetLimit(config.SyncS3MaxConcurrentUploads.Int())

	for _, fname := range blobs {
		g.Go(func() error {
			f, err := os.Open(fname)
			if err != nil {
				return err
			}
			defer f.Close()

			key := filepath.Join(s3c.baseDir, "blobs", filepath.Base(fname))

			mediaType := "application/vnd.docker.image.rootfs.diff.tar.gzip"

			if err := syncObject(
				ctx,
				s3c,
				key,
				aws.String(mediaType),
				f,
				false,
			); err != nil {
				return err
			}

			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return err
	}

	for _, fname := range manifests {
		g.Go(func() error {
			key := filepath.Join(s3c.baseDir, "manifests", filepath.Base(fname))

			b, err := os.ReadFile(fname)
			if err != nil {
				return err
			}

			var mwmt manifestWithMediaType

			if err := json.Unmarshal(b, &mwmt); err != nil {
				return err
			}

			if mwmt.MediaType == "" {
				mwmt.MediaType = "application/vnd.docker.distribution.manifest.v1+prettyjws"
			}

			if err := syncObject(
				ctx,
				s3c,
				key,
				aws.String(mwmt.MediaType),
				bytes.NewReader(b),
				strings.HasSuffix(key, fmt.Sprintf("/%s", tag)),
			); err != nil {
				return err
			}

			return nil
		})
	}
	if err := g.Wait(); err != nil {
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
	force bool,
) error {
	cacheKey := fmt.Sprintf("%s/%s", *s3c.bucket, key)

	if !force && config.SyncS3ObjectCacheEnabled.Bool() {
		if seem := objectCache.Has(cacheKey); seem {
			log.Debug().
				Str("bucket", *s3c.bucket).
				Str("key", key).
				Msg("Object seem recently, skipping upload")
			return nil
		}
	}

	exists, headMetadataDigest, err := s3ObjectExists(ctx, s3c.s3Session, s3c.bucket, key)
	if err != nil {
		return err
	}

	fname := path.Base(key)

	// Try to avoid uploading the object if it already exists
	if !force && exists && strings.HasPrefix(fname, "sha256:") && fname == headMetadataDigest {
		log.Debug().
			Str("bucket", *s3c.bucket).
			Str("key", key).
			Msg("Object already exists with same digest, skipping upload")

		if config.SyncS3ObjectCacheEnabled.Bool() {
			objectCache.Set(cacheKey, true, ttlcache.DefaultTTL)
		}
		return nil
	}

	// Blobs can be huge and we need an io.ReadSeeker, so we can't read them all into memory.
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
	if !force && calculatedDigest == headMetadataDigest {
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

	dataCounter := &s3DataCounter{
		ctx:  ctx,
		dest: s3c.dst,
		f:    tmpFile,
	}

	if _, err := s3c.uploader.Upload(ctx, &s3.PutObjectInput{
		Bucket:      s3c.bucket,
		Key:         aws.String(key),
		Body:        dataCounter,
		ACL:         awstypes.ObjectCannedACL(*s3c.acl),
		ContentType: contentType,
		ContentMD5:  aws.String(contentMD5),
		Metadata: map[string]string{
			"x-calculated-digest": calculatedDigest,
		},
	}); err != nil {
		return fmt.Errorf("failed to upload object: %w", err)
	}

	if config.SyncS3ObjectCacheEnabled.Bool() {
		objectCache.Set(cacheKey, true, ttlcache.DefaultTTL)
	}

	return nil
}

func s3ObjectExists(ctx context.Context, s3Session *s3.Client, bucket *string, key string) (bool, string, error) {
	head, err := s3Session.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: bucket,
		Key:    &key,
	})
	if err != nil {
		var nf *awstypes.NotFound
		if !errors.As(err, &nf) {
			return false, "", err
		}

		return false, "", nil
	}

	// R2 only supports MD5, so we need to check the custom X-Calculated-Digest metadata for the SHA256 hash
	if head != nil && head.Metadata != nil {
		if digest, ok := head.Metadata["x-calculated-digest"]; ok {
			return true, digest, nil
		}
	}

	return true, "", nil
}
