// Command api is the tenant-facing REST API (Gin).
package main

import (
	"context"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/ngocmx2104/market-sync/internal/adapters/mysqlstore"
	"github.com/ngocmx2104/market-sync/internal/platform"
)

func main() {
	log := platform.NewLogger("api")
	ctx, stop := platform.SignalContext()
	defer stop()

	store, err := mysqlstore.Open(ctx, platform.Env("MYSQL_DSN", "app:app@tcp(localhost:3306)/marketsync?parseTime=true"))
	if err != nil {
		log.Error("mysql", "err", err)
		os.Exit(1)
	}
	defer store.Close()

	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())

	r.GET("/healthz", func(c *gin.Context) { c.Status(http.StatusOK) })
	r.GET("/readyz", func(c *gin.Context) {
		pingCtx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()
		if err := store.Ping(pingCtx); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "mysql unavailable"})
			return
		}
		c.Status(http.StatusOK)
	})

	v1 := r.Group("/v1")
	// TODO(core): JWT middleware that puts the tenant in the context, and a
	// check that :tenant matches it (tenant isolation, no IDOR).
	v1.GET("/tenants/:tenant/orders", func(c *gin.Context) {
		// TODO(core): keyset pagination (?after=<cursor>&limit=), filters by
		// status and updated_at, deleted orders hidden by default.
		c.JSON(http.StatusNotImplemented, gin.H{"error": "not implemented"})
	})

	srv := &http.Server{
		Addr:              platform.Env("API_ADDR", ":8080"),
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
	}
	if err := platform.ServeHTTP(ctx, log, srv, 15*time.Second); err != nil {
		log.Error("server failed", "err", err)
		os.Exit(1)
	}
}
