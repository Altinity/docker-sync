package sync

import (
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type manifestWithMediaType struct {
	MediaType string `json:"mediaType"`
}

type s3Client struct {
	uploader  *manager.Uploader
	s3Session *s3.Client
	dst       string
	bucket    *string
	acl       *string
	baseDir   string
}
