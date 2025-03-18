package sync

import (
	"context"
	"testing"

	"github.com/Altinity/docker-sync/config"
	"github.com/containers/image/v5/types"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func setupTestData(t *testing.T) {
	if config.SyncRegistries == nil {
		config.SyncRegistries = config.NewKey(
			"sync.registries",
			config.WithValidRepositories(),
		)
	}

	mockRepos := []map[string]interface{}{
		{
			"name": "example",
			"url":  "https://example.com",
			"auth": map[string]interface{}{
				"username": "testuser",
				"password": "testpass",
			},
		},
		{
			"name": "ecr-private",
			"url":  "https://123456789012.dkr.ecr.us-east-1.amazonaws.com",
			"auth": map[string]interface{}{
				"helper": "ecr",
			},
		},
		{
			"name": "ecr-public",
			"url":  "https://public.ecr.aws",
			"auth": map[string]interface{}{
				"helper": "ecr-public",
			},
		},
	}

	viper.Set("sync.registries", mockRepos)

	if reloaded := config.SyncRegistries.Update(); reloaded != nil && reloaded.Error != nil {
		t.Fatalf("Failed to update SyncRegistries config: %v", reloaded.Error)
	}
}

func cleanupTestData(t *testing.T) {
	viper.Set("sync.registries", nil)
	if reloaded := config.SyncRegistries.Update(); reloaded != nil && reloaded.Error != nil {
		t.Logf("Failed to cleanup SyncRegistries config: %v", reloaded.Error)
	}
}

func TestGetObjectStorageAuth(t *testing.T) {
	setupAWSEnv(t)
	setupTestData(t)
	defer cleanupTestData(t)

	tests := []struct {
		name           string
		url            string
		expectedUser   string
		expectedPass   string
		expectedErrMsg string
	}{
		{
			name:           "Valid credentials",
			url:            "https://example.com",
			expectedUser:   "testuser",
			expectedPass:   "testpass",
			expectedErrMsg: "",
		},
		{
			name:           "Unknown repository",
			url:            "https://unknown.com",
			expectedUser:   "",
			expectedPass:   "",
			expectedErrMsg: "no auth found for https://unknown.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			username, password, err := getObjectStorageAuth(tt.url)

			assert.Equal(t, tt.expectedUser, username)
			assert.Equal(t, tt.expectedPass, password)

			if tt.expectedErrMsg != "" {
				assert.EqualError(t, err, tt.expectedErrMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestGetSkopeoAuth(t *testing.T) {
	setupAWSEnv(t)
	setupTestData(t)
	defer cleanupTestData(t)

	tests := []struct {
		name             string
		url              string
		imageName        string
		expectedAuthType string
		checkAuthConfig  func(*testing.T, *types.DockerAuthConfig)
	}{
		{
			name:             "Basic auth repository",
			url:              "https://example.com",
			imageName:        "test-image",
			expectedAuthType: "basic",
			checkAuthConfig: func(t *testing.T, auth *types.DockerAuthConfig) {
				assert.NotNil(t, auth)
				assert.Equal(t, "testuser", auth.Username)
				assert.Equal(t, "testpass", auth.Password)
			},
		},
		{
			name:             "ECR private repository",
			url:              "https://123456789012.dkr.ecr.us-east-1.amazonaws.com",
			imageName:        "test-image",
			expectedAuthType: "ecr",
			checkAuthConfig: func(t *testing.T, auth *types.DockerAuthConfig) {
				if auth == nil || (auth.Username == "" && auth.Password == "") {
					t.Log("ECR auth returned empty - this is expected if no valid AWS credentials are available")
					return
				}
				assert.NotNil(t, auth)
				assert.NotEmpty(t, auth.Username)
				assert.NotEmpty(t, auth.Password)
			},
		},
		{
			name:             "ECR public repository",
			url:              "https://public.ecr.aws",
			imageName:        "test-image",
			expectedAuthType: "ecr-public",
			checkAuthConfig: func(t *testing.T, auth *types.DockerAuthConfig) {
				if auth == nil || (auth.Username == "" && auth.Password == "") {
					t.Log("ECR public auth returned empty - this is expected if no valid AWS credentials are available")
					return
				}
				assert.NotNil(t, auth)
				assert.NotEmpty(t, auth.Username)
				assert.NotEmpty(t, auth.Password)
			},
		},
		{
			name:             "Unknown repository",
			url:              "https://unknown.com",
			imageName:        "test-image",
			expectedAuthType: "default",
			checkAuthConfig: func(t *testing.T, auth *types.DockerAuthConfig) {
				assert.Nil(t, auth)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			authConfig, authType := getSkopeoAuth(context.Background(), tt.url, tt.imageName)
			assert.Equal(t, tt.expectedAuthType, authType)
			tt.checkAuthConfig(t, authConfig)
		})
	}
}
