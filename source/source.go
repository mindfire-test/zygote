package source

import (
	"context"

	"github.com/mindfire/zygote/vfs"
)

type Source interface {
	Load(ctx context.Context, w *vfs.World) error
	Apply(ctx context.Context, w *vfs.World) error
}
