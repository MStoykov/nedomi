package contexts

import (
	"github.com/ironsmile/nedomi/types"
	"golang.org/x/net/context"
)

// The key type is unexported to prevent collisions with context keys defined in
// other packages.
type storageContextKey int

const sKey storageContextKey = 0

// NewStoragesContext returns a new Context carrying the map with the supplied storages.
func NewStoragesContext(ctx context.Context, storages map[string]types.Storage) context.Context {
	return context.WithValue(ctx, sKey, storages)
}

// GetStorages extracts the map of types.Storage objects, if present.
func GetStorages(ctx context.Context) (map[string]types.Storage, bool) {
	// ctx.Value returns nil if ctx has no value for the key;
	// the map[string]types.Storage type assertion returns ok=false for nil.
	storages, ok := ctx.Value(sKey).(map[string]types.Storage)
	return storages, ok
}
