package sync

import (
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
)

type manifestWithMediaType struct {
	MediaType string `json:"mediaType"`
}

type s3Client struct {
	uploader  *s3manager.Uploader
	s3Session *s3.S3
	dst       string
	bucket    *string
	acl       *string
	baseDir   string
}
