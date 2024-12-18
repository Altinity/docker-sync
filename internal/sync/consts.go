package sync

type RepositoryType string

const (
	S3CompatibleRepository RepositoryType = "s3"
	OCIRepository          RepositoryType = "oci"
)
