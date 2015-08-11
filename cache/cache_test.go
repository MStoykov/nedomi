package cache

import (
	"os"
	"testing"

	"github.com/ironsmile/nedomi/config"
)

func TestCreatingCacheMangers(t *testing.T) {
	cz := config.CacheZoneSection{
		ID:             "default",
		Path:           os.TempDir(),
		PartSize:       4123123,
		StorageObjects: 9813743,
	}

	if _, err := New("lru", &cz); err != nil {
		t.Errorf("Error when creating cache manager. %s", err)
	}
}

func TestCreatingBogusCacheMangerReturnsError(t *testing.T) {
	cz := config.CacheZoneSection{
		ID:             "default",
		Path:           os.TempDir(),
		PartSize:       4123123,
		StorageObjects: 9813743,
	}

	if _, err := New("bogus", &cz); err == nil {
		t.Error("Expected an error when creating bogus manager but got none")
	}
}
