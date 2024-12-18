package config

import "time"

var (
	// SyncS3MaxConcurrentUploads is the maximum number of concurrent uploads to S3.
	SyncS3MaxConcurrentUploads = NewKey("sync.s3.maxConcurrentUploads",
		WithDefaultValue(10),
		WithValidInt())

	// SyncS3ObjectCacheEnabled enables S3 object-seem cache.
	SyncS3ObjectCacheEnabled = NewKey("sync.s3.objectCache.enabled",
		WithDefaultValue(true),
		WithValidBool())

	// SyncS3ObjectCacheExpirationTime is the expiration time for S3 object cache.
	SyncS3ObjectCacheExpirationTime = NewKey("sync.s3.objectCache.expirationTime",
		WithDefaultValue(10*time.Minute),
		WithValidDuration())

	// SyncS3ObjectCacheCapacity is the maximum number of entries the S3 object cache can hold.
	SyncS3ObjectCacheCapacity = NewKey("sync.s3.objectCache.capacity",
		WithDefaultValue(1000),
		WithValidInt())
)
