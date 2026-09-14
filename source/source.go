// Package source provides source.
package source

import (
	"context"

	"github.com/mindfire/zygote/vfs"
)

// Source defines the interface.
type Source interface {
	Load(ctx context.Context, w *vfs.World) error
	Apply(ctx context.Context, w *vfs.World) error
}
