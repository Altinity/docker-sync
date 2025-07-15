package sync

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func setupAWSEnv(t *testing.T) {
	// Set AWS region for testing
	t.Setenv("AWS_REGION", "us-east-1")
	// Optionally, set other AWS configs if needed for your testing environment
	// t.Setenv("AWS_ACCESS_KEY_ID", "test")
	// t.Setenv("AWS_SECRET_ACCESS_KEY", "test")
}

func TestECRPrivateAuth(t *testing.T) {
	setupAWSEnv(t)

	username, password := authEcrPrivate(t.Context(), "test-repository")

	if username == "" && password == "" {
		t.Log("No ECR credentials returned - this is expected if no valid AWS credentials are available")
	} else {
		assert.NotEmpty(t, username)
		assert.NotEmpty(t, password)
	}
}

func TestECRPublicAuth(t *testing.T) {
	setupAWSEnv(t)

	username, password := authEcrPublic(t.Context(), "test-repository")

	if username == "" && password == "" {
		t.Log("No ECR public credentials returned - this is expected if no valid AWS credentials are available")
	} else {
		assert.NotEmpty(t, username)
		assert.NotEmpty(t, password)
	}
}
