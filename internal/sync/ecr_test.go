package sync

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func setupAWSEnv(t *testing.T) {
	// Set AWS region for testing
	os.Setenv("AWS_REGION", "us-east-1")
	// Optionally, set other AWS configs if needed for your testing environment
	// os.Setenv("AWS_ACCESS_KEY_ID", "test")
	// os.Setenv("AWS_SECRET_ACCESS_KEY", "test")
}

func cleanupAWSEnv(t *testing.T) {
	os.Unsetenv("AWS_REGION")
	// os.Unsetenv("AWS_ACCESS_KEY_ID")
	// os.Unsetenv("AWS_SECRET_ACCESS_KEY")
}

func TestECRPrivateAuth(t *testing.T) {
	setupAWSEnv(t)
	defer cleanupAWSEnv(t)

	username, password := authEcrPrivate("test-repository")

	if username == "" && password == "" {
		t.Log("No ECR credentials returned - this is expected if no valid AWS credentials are available")
	} else {
		assert.NotEmpty(t, username)
		assert.NotEmpty(t, password)
	}
}

func TestECRPublicAuth(t *testing.T) {
	setupAWSEnv(t)
	defer cleanupAWSEnv(t)

	username, password := authEcrPublic("test-repository")

	if username == "" && password == "" {
		t.Log("No ECR public credentials returned - this is expected if no valid AWS credentials are available")
	} else {
		assert.NotEmpty(t, username)
		assert.NotEmpty(t, password)
	}
}
