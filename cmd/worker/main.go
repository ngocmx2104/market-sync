// Command worker consumes sync jobs from SQS and stores orders in MySQL.
package main

import (
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/sqs"

	"github.com/ngocmx2104/market-sync/internal/adapters/dynamostore"
	"github.com/ngocmx2104/market-sync/internal/adapters/marketclient"
	"github.com/ngocmx2104/market-sync/internal/adapters/mysqlstore"
	"github.com/ngocmx2104/market-sync/internal/adapters/sqsqueue"
	"github.com/ngocmx2104/market-sync/internal/app/syncer"
	"github.com/ngocmx2104/market-sync/internal/platform"
)

func main() {
	log := platform.NewLogger("worker")
	ctx, stop := platform.SignalContext()
	defer stop()

	store, err := mysqlstore.Open(ctx, platform.Env("MYSQL_DSN", "app:app@tcp(localhost:3306)/marketsync?parseTime=true"))
	if err != nil {
		log.Error("mysql", "err", err)
		os.Exit(1)
	}
	defer store.Close()

	awsCfg, err := platform.AWSConfig(ctx)
	if err != nil {
		log.Error("aws config", "err", err)
		os.Exit(1)
	}

	w := &syncer.Worker{
		Queue:       sqsqueue.New(sqs.NewFromConfig(awsCfg), platform.Env("SYNC_QUEUE_URL", "http://localhost:9324/000000000000/sync-jobs")),
		Source:      marketclient.New(platform.Env("FAKE_MARKET_URL", "http://localhost:8090"), nil),
		Orders:      store,
		Quarantine:  store,
		Cursors:     dynamostore.NewCursorStore(dynamodb.NewFromConfig(awsCfg), platform.Env("CURSOR_TABLE", "sync_cursors")),
		Log:         log,
		PageSize:    platform.EnvInt("PAGE_SIZE", 200),
		Concurrency: platform.EnvInt("WORKER_CONCURRENCY", 4),
		PollWait:    platform.EnvDuration("POLL_WAIT", 10*time.Second),
	}
	log.Info("worker started")
	if err := w.Run(ctx); err != nil {
		log.Error("worker stopped", "err", err)
		os.Exit(1)
	}
	log.Info("worker stopped")
}
