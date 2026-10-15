package worker

import (
	"bytes"
	"context"
	"io"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dexisback/YellowBird/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockStorageForCache struct {
	downloadCount int32
	content       []byte
}

func (m *mockStorageForCache) Upload(ctx context.Context, input storage.UploadInput) (*storage.UploadResult, error) {
	return nil, nil
}

func (m *mockStorageForCache) Delete(ctx context.Context, storageKey string) error {
	return nil
}

func (m *mockStorageForCache) GetURL(ctx context.Context, storageKey string) (string, error) {
	return "", nil
}

func (m *mockStorageForCache) Download(ctx context.Context, storageKey, mimeType string) (io.ReadCloser, error) {
	atomic.AddInt32(&m.downloadCount, 1)
	time.Sleep(20 * time.Millisecond) // simulate network latency
	return io.NopCloser(bytes.NewReader(m.content)), nil
}

func TestSourceCacheSingleDownload(t *testing.T) {
	tempDir := t.TempDir()
	mockSt := &mockStorageForCache{content: []byte("sample-video-bytes")}
	cache := NewSourceCache(mockSt, tempDir, 10*time.Minute)
	defer cache.Close()

	ctx := context.Background()
	const numCallers = 5

	var wg sync.WaitGroup
	paths := make([]string, numCallers)

	for i := 0; i < numCallers; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			path, release, err := cache.Get(ctx, "media/source.mp4", "video/mp4")
			require.NoError(t, err)
			defer release()
			paths[idx] = path
		}(i)
	}

	wg.Wait()

	// Assert only 1 download was performed over the network for all 5 callers
	assert.Equal(t, int32(1), atomic.LoadInt32(&mockSt.downloadCount), "expected singleflight download deduplication")

	// Assert all callers got the same valid file
	for _, p := range paths {
		assert.Equal(t, paths[0], p)
		content, err := os.ReadFile(p)
		require.NoError(t, err)
		assert.Equal(t, []byte("sample-video-bytes"), content)
	}
}

func TestSourceCacheEviction(t *testing.T) {
	tempDir := t.TempDir()
	mockSt := &mockStorageForCache{content: []byte("temporary-video-bytes")}
	cache := NewSourceCache(mockSt, tempDir, 50*time.Millisecond)
	defer cache.Close()

	ctx := context.Background()
	path, release, err := cache.Get(ctx, "media/expired.mp4", "video/mp4")
	require.NoError(t, err)

	_, err = os.Stat(path)
	require.NoError(t, err)

	// Release reference
	release()

	// Wait past TTL
	time.Sleep(100 * time.Millisecond)
	cache.evictExpired()

	// The file should now be deleted from disk
	_, err = os.Stat(path)
	assert.True(t, os.IsNotExist(err), "file should be removed after eviction")
}
