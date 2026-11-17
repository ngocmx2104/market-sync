// Command reconciler periodically re-checks open orders against the source.
package main

import (
	"errors"
	"os"
	"strings"
	"time"

	"github.com/ngocmx2104/market-sync/internal/adapters/marketclient"
	"github.com/ngocmx2104/market-sync/internal/adapters/mysqlstore"
	"github.com/ngocmx2104/market-sync/internal/app/reconcile"
	"github.com/ngocmx2104/market-sync/internal/platform"
)

func main() {
	log := platform.NewLogger("reconciler")
	ctx, stop := platform.SignalContext()
	defer stop()

	store, err := mysqlstore.Open(ctx, platform.Env("MYSQL_DSN", "app:app@tcp(localhost:3306)/marketsync?parseTime=true"))
	if err != nil {
		log.Error("mysql", "err", err)
		os.Exit(1)
	}
	defer store.Close()

	r := &reconcile.Reconciler{
		Source:          marketclient.New(platform.Env("FAKE_MARKET_URL", "http://localhost:8090"), nil),
		Orders:          store,
		Log:             log,
		MaxMissingRatio: 0.2,
	}
	tenants := strings.Split(platform.Env("TENANTS", "shop-a"), ",")
	every := platform.EnvDuration("RECONCILE_EVERY", time.Hour)

	ticker := time.NewTicker(every)
	defer ticker.Stop()
	for {
		for _, t := range tenants {
			rep, err := r.RunOnce(ctx, t)
			var guard *reconcile.ErrGuardTripped
			switch {
			case errors.As(err, &guard):
				log.Error("guard tripped, nothing marked", "tenant", t, "checked", guard.Checked, "missing", guard.Missing)
			case err != nil:
				log.Error("reconcile failed", "tenant", t, "err", err)
			default:
				log.Info("reconciled", "tenant", t, "checked", rep.Checked, "refreshed", rep.Refreshed, "deleted", rep.Deleted)
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
