//go:build integration

package adapters_test

import (
	"context"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/sqs"

	"github.com/ngocmx2104/market-sync/internal/adapters/sqsqueue"
	"github.com/ngocmx2104/market-sync/internal/domain"
	"github.com/ngocmx2104/market-sync/internal/platform"
)

// Needs `make up` (ElasticMQ on :9324) and the AWS_* variables the Makefile exports.
func TestSQSRoundTrip(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cfg, err := platform.AWSConfig(ctx)
	if err != nil {
		t.Fatal(err)
	}
	q := sqsqueue.New(sqs.NewFromConfig(cfg), platform.Env("SYNC_QUEUE_URL", "http://localhost:9324/000000000000/sync-jobs"))

	job := domain.SyncJob{
		JobID:    "it-" + time.Now().Format("150405.000"),
		TenantID: "shop-it",
		Source:   "fake-market",
		Resource: "orders",
		Window:   domain.Window{From: time.Now().Add(-time.Hour).UTC(), To: time.Now().UTC()},
	}
	if err := q.Enqueue(ctx, job); err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		msgs, err := q.Receive(ctx, 10, 2*time.Second)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range msgs {
			if err := q.Ack(ctx, m); err != nil {
				t.Fatal(err)
			}
			if m.Job.JobID == job.JobID {
				if m.ReceiveCount != 1 {
					t.Errorf("ReceiveCount = %d, want 1", m.ReceiveCount)
				}
				return
			}
		}
	}
	t.Fatalf("job %s not received", job.JobID)
}
