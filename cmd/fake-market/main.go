// Command fake-market serves the fake marketplace API used by local runs and
// integration tests.
package main

import (
	"net/http"
	"os"
	"time"

	"github.com/ngocmx2104/market-sync/internal/fakemarket"
	"github.com/ngocmx2104/market-sync/internal/platform"
)

func main() {
	log := platform.NewLogger("fake-market")
	ctx, stop := platform.SignalContext()
	defer stop()

	srv := fakemarket.NewServer(platform.EnvInt("FAKE_MARKET_SEED_ORDERS", 1000), time.Now)
	httpSrv := &http.Server{
		Addr:              platform.Env("FAKE_MARKET_ADDR", ":8090"),
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	if err := platform.ServeHTTP(ctx, log, httpSrv, 10*time.Second); err != nil {
		log.Error("server failed", "err", err)
		os.Exit(1)
	}
}
