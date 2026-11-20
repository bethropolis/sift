package app

import (
	"testing"

	"github.com/bethropolis/sift/internal/config"
)

func TestNewContentCacheHonorsNoCache(t *testing.T) {
	cfg := config.New()
	cfg.NoCache = true
	if NewContentCacheWithDisk(cfg).HasDisk() {
		t.Fatal("disk cache attached despite NoCache")
	}
}

func TestNoCacheEnvironment(t *testing.T) {
	t.Setenv("SIFT_NO_CACHE", "1")
	if !config.New().NoCache {
		t.Fatal("SIFT_NO_CACHE=1 did not disable the cache")
	}
	t.Setenv("SIFT_NO_CACHE", "")
	if config.New().NoCache {
		t.Fatal("empty SIFT_NO_CACHE disabled the cache")
	}
}
