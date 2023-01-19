package server

import (
	"context"

	"github.com/kasterism/astertower/pkg/server/router"
)

func Start(ctx context.Context, listen string) {
	// Launch Router
	router.NewRouter(listen)
}
