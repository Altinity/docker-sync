package sync

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Altinity/docker-sync/config"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/jellydator/ttlcache/v3"
)

const testBucket = "cr-enam-test"

func init() {
	// Initialize the object cache if it hasn't been initialized
	if objectCache == nil {
		objectCache = ttlcache.New[string, bool](
			ttlcache.WithTTL[string, bool](5 * time.Minute),
		)
	}
}

func TestGetR2Session(t *testing.T) {
	// Add R2 specific repository to the config
	mockRepos := []map[string]interface{}{
		{
			"name": "r2-test",
			// Include the full URL path that will be used for authentication
			"url": "r2:account-id:bucket-name",
			"auth": map[string]interface{}{
				"username": "r2-access-key",
				"password": "r2-secret-key",
			},
		},
	}

	// Set up the configuration
	viper.Set("sync.registries", mockRepos)
	if config.SyncRegistries == nil {
		config.SyncRegistries = config.NewKey(
			"sync.registries",
			config.WithValidRepositories(),
		)
	}
	if reloaded := config.SyncRegistries.Update(); reloaded != nil && reloaded.Error != nil {
		t.Fatalf("Failed to update SyncRegistries config: %v", reloaded.Error)
	}

	tests := []struct {
		name        string
		url         string
		wantErr     bool
		errContains string
		checkResult func(*testing.T, *s3.Client, *string)
	}{
		{
			name:    "Valid R2 URL",
			url:     "r2:account-id:bucket-name:image",
			wantErr: false,
			checkResult: func(t *testing.T, client *s3.Client, bucket *string) {
				assert.NotNil(t, client)
				assert.NotNil(t, bucket)
				assert.Equal(t, "bucket-name", *bucket)

				// Verify the client configuration
				cfg := client.Options()
				assert.Equal(t, "https://account-id.r2.cloudflarestorage.com", *cfg.BaseEndpoint)
				assert.Equal(t, "us-east-1", cfg.Region)

				// Verify credentials
				creds, err := cfg.Credentials.Retrieve(context.Background())
				assert.NoError(t, err)
				assert.Equal(t, "r2-access-key", creds.AccessKeyID)
				assert.Equal(t, "r2-secret-key", creds.SecretAccessKey)
				assert.Empty(t, creds.SessionToken)
			},
		},
		{
			name:        "Invalid R2 URL format",
			url:         "r2:invalid:url",
			wantErr:     true,
			errContains: "invalid R2 destination",
		},
		{
			name:        "Unknown repository",
			url:         "r2:unknown:bucket:image",
			wantErr:     true,
			errContains: "no auth found",
		},
		{
			name:        "Empty URL",
			url:         "",
			wantErr:     true,
			errContains: "invalid R2 destination",
		},
		{
			name:        "Wrong protocol",
			url:         "s3:account-id:bucket:image",
			wantErr:     true,
			errContains: "invalid protocol: s3, expected r2",
		},
		{
			name:        "Extra segments",
			url:         "r2:account:bucket:image:extra",
			wantErr:     true,
			errContains: "invalid R2 destination",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, bucket, err := getR2Session(tt.url)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
				assert.Nil(t, client)
				assert.Nil(t, bucket)
				return
			}

			assert.NoError(t, err)
			tt.checkResult(t, client, bucket)
		})
	}
}

// hasR2Credentials checks if all required R2 environment variables are set.
func hasR2Credentials() bool {
	accountID := os.Getenv("R2_ACCOUNT_ID")
	accessKeyID := os.Getenv("R2_ACCESS_KEY_ID")
	secretAccessKey := os.Getenv("R2_SECRET_ACCESS_KEY")
	return accountID != "" && accessKeyID != "" && secretAccessKey != ""
}

// skipIfNoR2Credentials skips the test if R2 credentials are not available.
func skipIfNoR2Credentials(t *testing.T) {
	if !hasR2Credentials() {
		t.Skip("Skipping test: R2 credentials not available")
	}
}

func setupR2TestConfig(t *testing.T) {
	skipIfNoR2Credentials(t)

	accountID := os.Getenv("R2_ACCOUNT_ID")
	accessKeyID := os.Getenv("R2_ACCESS_KEY_ID")
	secretAccessKey := os.Getenv("R2_SECRET_ACCESS_KEY")

	// Add R2 specific repository to the config
	mockRepos := []map[string]interface{}{
		{
			"name": "r2-test",
			"url":  fmt.Sprintf("r2:%s:%s", accountID, testBucket),
			"auth": map[string]interface{}{
				"username": accessKeyID,
				"password": secretAccessKey,
			},
		},
	}

	// Set up the configuration
	viper.Set("sync.registries", mockRepos)
	if config.SyncRegistries == nil {
		config.SyncRegistries = config.NewKey(
			"sync.registries",
			config.WithValidRepositories(),
		)
	}
	if reloaded := config.SyncRegistries.Update(); reloaded != nil && reloaded.Error != nil {
		t.Fatalf("Failed to update SyncRegistries config: %v", reloaded.Error)
	}
}

func TestR2Integration(t *testing.T) {
	// Basic tests that don't require credentials
	t.Run("basic", func(t *testing.T) {
		tests := []struct {
			name        string
			url         string
			wantErr     bool
			errContains string
		}{
			{
				name:        "Invalid R2 URL format",
				url:         "r2:invalid:url",
				wantErr:     true,
				errContains: "invalid R2 destination",
			},
			{
				name:        "Empty URL",
				url:         "",
				wantErr:     true,
				errContains: "invalid R2 destination",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				client, bucket, err := getR2Session(tt.url)
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
				assert.Nil(t, client)
				assert.Nil(t, bucket)
			})
		}
	})

	// Integration tests that require credentials
	t.Run("integration", func(t *testing.T) {
		skipIfNoR2Credentials(t)
		setupR2TestConfig(t)

		accountID := os.Getenv("R2_ACCOUNT_ID")
		url := fmt.Sprintf("r2:%s:%s:image", accountID, testBucket)

		client, bucket, err := getR2Session(url)
		assert.NoError(t, err)
		assert.NotNil(t, client)
		assert.NotNil(t, bucket)
		assert.Equal(t, testBucket, *bucket)

		// Verify the client configuration
		cfg := client.Options()
		assert.Equal(t, "us-east-1", cfg.Region)
		assert.Equal(t, fmt.Sprintf("https://%s.r2.cloudflarestorage.com", accountID), *cfg.BaseEndpoint)

		// Verify credentials
		creds, err := cfg.Credentials.Retrieve(context.Background())
		assert.NoError(t, err)
		assert.Equal(t, os.Getenv("R2_ACCESS_KEY_ID"), creds.AccessKeyID)
		assert.Equal(t, os.Getenv("R2_SECRET_ACCESS_KEY"), creds.SecretAccessKey)
	})
}

func TestR2SyncObject(t *testing.T) {
	skipIfNoR2Credentials(t)
	setupR2TestConfig(t)

	accountID := os.Getenv("R2_ACCOUNT_ID")
	s3Session, bucket, err := getR2Session(fmt.Sprintf("r2:%s:%s:test", accountID, testBucket))
	if err != nil {
		t.Fatal(err)
	}

	// Create the s3Client with all required components
	s3c := &s3Client{
		bucket:    bucket,
		acl:       aws.String("public-read"),
		baseDir:   "test",
		dst:       fmt.Sprintf("r2:%s:%s:test", accountID, testBucket),
		s3Session: s3Session,
		uploader:  manager.NewUploader(s3Session), // This was missing
	}

	content := "test content"
	tests := []struct {
		name        string
		key         string
		contentType string
		reader      io.Reader
		setupCache  bool
		wantErr     bool
	}{
		{
			name:        "New object upload",
			key:         "test/newfile.txt",
			contentType: "text/plain",
			reader:      strings.NewReader(content),
			wantErr:     false,
		},
		{
			name:        "Cached object upload",
			key:         "test/cachedfile.txt",
			contentType: "text/plain",
			reader:      strings.NewReader(content),
			setupCache:  true,
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a new reader for each test to ensure we're starting from the beginning
			reader := strings.NewReader(content)

			if tt.setupCache {
				cacheKey := fmt.Sprintf("%s/%s", *s3c.bucket, tt.key)
				objectCache.Set(cacheKey, true, ttlcache.DefaultTTL)
			}

			err := syncObject(
				context.Background(),
				s3c,
				tt.key,
				aws.String(tt.contentType),
				reader,
				false,
			)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)

				// Verify the object exists
				exists, digest, err := s3ObjectExists(context.Background(), s3Session, bucket, tt.key)
				assert.NoError(t, err)
				assert.True(t, exists)
				assert.NotEmpty(t, digest)
				assert.True(t, strings.HasPrefix(digest, "sha256:"))

				// Optional: verify content
				output, err := s3Session.GetObject(context.Background(), &s3.GetObjectInput{
					Bucket: bucket,
					Key:    aws.String(tt.key),
				})
				assert.NoError(t, err)

				if output.Body != nil {
					defer output.Body.Close()
					body, err := io.ReadAll(output.Body)
					assert.NoError(t, err)
					assert.Equal(t, content, string(body))
				}
			}

			// Cleanup
			_, err = s3Session.DeleteObject(context.Background(), &s3.DeleteObjectInput{
				Bucket: bucket,
				Key:    aws.String(tt.key),
			})
			if err != nil {
				t.Logf("Failed to cleanup test object: %v", err)
			}
		})
	}
}

func TestR2ObjectOperations(t *testing.T) {
	skipIfNoR2Credentials(t)
	setupR2TestConfig(t)

	accountID := os.Getenv("R2_ACCOUNT_ID")
	s3Session, bucket, err := getR2Session(fmt.Sprintf("r2:%s:%s:test", accountID, testBucket))
	if err != nil {
		t.Fatal(err)
	}

	testKey := "test/r2-object-test.txt"
	testContent := "test content for R2"

	t.Run("object lifecycle", func(t *testing.T) {
		// Clean up any existing object
		_, _ = s3Session.DeleteObject(context.Background(), &s3.DeleteObjectInput{
			Bucket: bucket,
			Key:    aws.String(testKey),
		})

		// Test object doesn't exist initially
		exists, digest, err := s3ObjectExists(context.Background(), s3Session, bucket, testKey)
		assert.NoError(t, err)
		assert.False(t, exists)
		assert.Empty(t, digest)

		// Create uploader
		uploader := manager.NewUploader(s3Session)

		// Upload object
		s3c := &s3Client{
			bucket:    bucket,
			acl:       aws.String("public-read"),
			baseDir:   "test",
			dst:       fmt.Sprintf("r2:%s:%s:test", accountID, testBucket),
			s3Session: s3Session,
			uploader:  uploader, // Make sure uploader is set
		}

		err = syncObject(
			context.Background(),
			s3c,
			testKey,
			aws.String("text/plain"),
			strings.NewReader(testContent),
			false,
		)
		assert.NoError(t, err)

		// Verify object exists
		exists, digest, err = s3ObjectExists(context.Background(), s3Session, bucket, testKey)
		assert.NoError(t, err)
		assert.True(t, exists)
		assert.NotEmpty(t, digest)
		assert.True(t, strings.HasPrefix(digest, "sha256:"))

		// Verify content
		output, err := s3Session.GetObject(context.Background(), &s3.GetObjectInput{
			Bucket: bucket,
			Key:    aws.String(testKey),
		})
		assert.NoError(t, err)

		if output.Body != nil {
			defer output.Body.Close()
			body, err := io.ReadAll(output.Body)
			assert.NoError(t, err)
			assert.Equal(t, testContent, string(body))
		}

		// Clean up
		_, err = s3Session.DeleteObject(context.Background(), &s3.DeleteObjectInput{
			Bucket: bucket,
			Key:    aws.String(testKey),
		})
		assert.NoError(t, err)
	})
}
