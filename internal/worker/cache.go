package worker

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/dexisback/YellowBird/internal/storage"
	"golang.org/x/sync/singleflight"
)

const (
	defaultCacheTTL     = 15 * time.Minute
	cleanupInterval     = 2 * time.Minute
	defaultCacheDirName = "yellowbird-cache"
)

// SourceCache manages local disk caching of source media assets so sibling jobs
// (e.g. thumbnail, preview, and multiple transcode resolutions for the same video)
// do not re-download the identical source file from remote storage multiple times.
type SourceCache struct {
	storage    storage.Storage
	cacheDir   string
	ttl        time.Duration
	sf         singleflight.Group
	mu         sync.Mutex
	refs       map[string]int
	lastAccess map[string]time.Time
	stopClean  chan struct{}
	closeOnce  sync.Once
}

// NewSourceCache creates a new local source cache.
func NewSourceCache(st storage.Storage, cacheDir string, ttl ...time.Duration) *SourceCache {
	if cacheDir == "" {
		cacheDir = filepath.Join(os.TempDir(), defaultCacheDirName)
	}
	_ = os.MkdirAll(cacheDir, 0755)

	cacheTTL := defaultCacheTTL
	if len(ttl) > 0 && ttl[0] > 0 {
		cacheTTL = ttl[0]
	}

	sc := &SourceCache{
		storage:    st,
		cacheDir:   cacheDir,
		ttl:        cacheTTL,
		refs:       make(map[string]int),
		lastAccess: make(map[string]time.Time),
		stopClean:  make(chan struct{}),
	}

	go sc.startEvictionLoop()

	return sc
}

// Get ensures the source media for storageKey is available on local disk.
// Concurrent calls for the same storageKey are deduplicated via singleflight,
// executing only 1 remote download. Returns the local file path and a release func.
func (c *SourceCache) Get(ctx context.Context, storageKey, mimeType string) (string, func(), error) {
	if c == nil || c.storage == nil {
		return "", func() {}, fmt.Errorf("source cache or storage is not configured")
	}

	hash := sha256Hex(storageKey)
	targetPath := filepath.Join(c.cacheDir, hash)

	val, err, _ := c.sf.Do(storageKey, func() (interface{}, error) {
		// Check if file already exists and is not empty.
		if fi, err := os.Stat(targetPath); err == nil && fi.Size() > 0 {
			return targetPath, nil
		}

		source, err := c.storage.Download(ctx, storageKey, mimeType)
		if err != nil {
			return "", fmt.Errorf("failed to download source media: %w", err)
		}
		defer source.Close()

		tempPath := fmt.Sprintf("%s.downloading-%d", targetPath, time.Now().UnixNano())
		tempFile, err := os.Create(tempPath)
		if err != nil {
			return "", fmt.Errorf("failed to create cache temp file: %w", err)
		}

		if _, err := io.Copy(tempFile, source); err != nil {
			_ = tempFile.Close()
			_ = os.Remove(tempPath)
			return "", fmt.Errorf("failed to write source media to cache: %w", err)
		}

		if err := tempFile.Close(); err != nil {
			_ = os.Remove(tempPath)
			return "", fmt.Errorf("failed to close cache temp file: %w", err)
		}

		if err := os.Rename(tempPath, targetPath); err != nil {
			_ = os.Remove(tempPath)
			return "", fmt.Errorf("failed to commit cached source file: %w", err)
		}

		return targetPath, nil
	})

	if err != nil {
		return "", func() {}, err
	}

	path := val.(string)

	c.mu.Lock()
	c.refs[path]++
	c.lastAccess[path] = time.Now()
	c.mu.Unlock()

	release := func() {
		c.mu.Lock()
		defer c.mu.Unlock()
		if c.refs[path] > 0 {
			c.refs[path]--
		}
		c.lastAccess[path] = time.Now()
	}

	return path, release, nil
}

func (c *SourceCache) startEvictionLoop() {
	ticker := time.NewTicker(cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.evictExpired()
		case <-c.stopClean:
			return
		}
	}
}

func (c *SourceCache) evictExpired() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	for path, last := range c.lastAccess {
		if c.refs[path] <= 0 && now.Sub(last) > c.ttl {
			_ = os.Remove(path)
			delete(c.refs, path)
			delete(c.lastAccess, path)
		}
	}
}

// Close stops background eviction and cleans up any unreferenced cache files.
func (c *SourceCache) Close() error {
	c.closeOnce.Do(func() {
		close(c.stopClean)
	})

	c.mu.Lock()
	defer c.mu.Unlock()

	for path := range c.lastAccess {
		if c.refs[path] <= 0 {
			_ = os.Remove(path)
			delete(c.refs, path)
			delete(c.lastAccess, path)
		}
	}

	return nil
}

func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}
