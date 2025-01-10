package sync

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func getRepositoryType(dst string) RepositoryType {
	fields := strings.Split(dst, ":")

	// If destination has format <type>:<region/endpoint>:<bucket>:<image>, then it's an S3-compatible storage
	if len(fields) == 4 {
		return S3CompatibleRepository
	}

	return OCIRepository
}

func shamove(baseDir string, oldPath string, folder string) (string, error) {
	f, err := os.Open(oldPath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	digest := fmt.Sprintf("sha256:%x", h.Sum(nil))

	newPath := filepath.Join(baseDir, folder, digest)

	if err := os.Rename(oldPath, newPath); err != nil {
		return "", err
	}

	return newPath, nil
}
