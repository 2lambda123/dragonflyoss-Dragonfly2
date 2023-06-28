package cache

import (
	"fmt"
	"time"

	"d7y.io/dragonfly/v2/manager/config"
	"d7y.io/dragonfly/v2/manager/database"
	"github.com/go-redis/cache/v8"
)

const (
	CDNNamespace        = "cdn"
	SchedulerNamespace  = "scheduler"
	SchedulersNamespace = "schedulers"
)

type Cache struct {
	*cache.Cache
	TTL time.Duration
}

// New cache instance
func New(cfg *config.Config) *Cache {
	var localCache *cache.TinyLFU
	if cfg.Cache != nil {
		localCache = cache.NewTinyLFU(cfg.Cache.Local.Size, cfg.Cache.Local.TTL)
	}

	// If the attribute TTL of cache.Item(cache's instance) is 0, redis expiration time is 1 hour.
	// cfg.TTL Set the expiration time of TinyLFU.
	return &Cache{
		Cache: cache.New(&cache.Options{
			Redis:      database.NewRedis(cfg.Database.Redis),
			LocalCache: localCache,
		}),
		TTL: cfg.Cache.Redis.TTL,
	}
}

func MakeCacheKey(namespace string, id string) string {
	return fmt.Sprintf("manager:%s:%s", namespace, id)
}

func MakeCDNCacheKey(hostname string, clusterID uint) string {
	return MakeCacheKey(CDNNamespace, fmt.Sprintf("%s-%d", hostname, clusterID))
}

func MakeSchedulerCacheKey(hostname string, clusterID uint) string {
	return MakeCacheKey(SchedulerNamespace, fmt.Sprintf("%s-%d", hostname, clusterID))
}

func MakeSchedulersCacheKey(hostname string) string {
	return MakeCacheKey(SchedulersNamespace, hostname)
}
