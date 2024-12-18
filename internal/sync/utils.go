package sync

import (
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
