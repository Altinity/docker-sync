package sync

import (
	"context"

	"github.com/Altinity/docker-sync/config"
	"github.com/jellydator/ttlcache/v3"
	"github.com/rs/zerolog/log"
)

// Cache already seem objects for a short time to avoid excessive S3 HEAD requests.
var objectCache *ttlcache.Cache[string, bool]

func init() {
	objectCache = ttlcache.New(
		ttlcache.WithTTL[string, bool](config.SyncS3ObjectCacheExpirationTime.Duration()),
		ttlcache.WithCapacity[string, bool](config.SyncS3ObjectCacheCapacity.UInt64()),
	)

	objectCache.OnEviction(func(ctx context.Context, reason ttlcache.EvictionReason, item *ttlcache.Item[string, bool]) {
		log.Debug().
			Str("key", item.Key()).
			Msg("Evicted object from cache")
	})

	if config.SyncS3ObjectCacheEnabled.Bool() {
		objectCache.Start()
	}
}
