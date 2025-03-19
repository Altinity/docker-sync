package sync

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestGetRepositoryType(t *testing.T) {
	tests := []struct {
		dst      string
		expected RepositoryType
	}{
		{"s3:us-east-1:mybucket:myimage", S3CompatibleRepository},
		{"docker.io/library/nginx", OCIRepository},
	}

	for _, test := range tests {
		result := getRepositoryType(test.dst)
		if result != test.expected {
			t.Errorf("getRepositoryType(%q) = %v; want %v", test.dst, result, test.expected)
		}
	}
}

func TestShamove(t *testing.T) {
	// Setup temporary base directory for testing
	baseDir := t.TempDir()

	// Create a temporary file for testing
	oldFile, err := os.CreateTemp(t.TempDir(), "shamove_old")
	if err != nil {
		t.Fatalf("unable to create temp file: %v", err)
	}
	defer os.Remove(oldFile.Name())

	content := []byte("this is a test file")
	if _, err := oldFile.Write(content); err != nil {
		t.Fatalf("unable to write to temp file: %v", err)
	}
	if err := oldFile.Close(); err != nil {
		t.Fatalf("unable to close temp file: %v", err)
	}

	folder := "dest"
	// Ensure the destination directory exists
	destDir := filepath.Join(baseDir, folder)
	if err := os.MkdirAll(destDir, os.ModePerm); err != nil {
		t.Fatalf("unable to create destination directory: %v", err)
	}

	newPath, err := shamove(baseDir, oldFile.Name(), folder)
	if err != nil {
		t.Fatalf("shamove failed: %v", err)
	}

	// Calculate expected digest
	h := sha256.New()
	h.Write(content)
	expectedDigest := fmt.Sprintf("sha256:%x", h.Sum(nil))

	expectedNewPath := filepath.Join(baseDir, folder, expectedDigest)

	if newPath != expectedNewPath {
		t.Errorf("shamove path = %q; want %q", newPath, expectedNewPath)
	}

	// Check if the file exists at new path
	if _, err := os.Stat(newPath); os.IsNotExist(err) {
		t.Errorf("file not found at new path: %q", newPath)
	}
}
