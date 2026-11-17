// Command control serves the internal gRPC operator API.
package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"log/slog"
	"net"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	controlv1 "github.com/ngocmx2104/market-sync/gen/control/v1"
	"github.com/ngocmx2104/market-sync/internal/adapters/dynamostore"
	"github.com/ngocmx2104/market-sync/internal/adapters/sqsqueue"
	"github.com/ngocmx2104/market-sync/internal/domain"
	"github.com/ngocmx2104/market-sync/internal/platform"
	"github.com/ngocmx2104/market-sync/internal/ports"
)

type server struct {
	controlv1.UnimplementedControlServiceServer
	queue   ports.Queue
	cursors ports.CursorStore
}

func (s *server) TriggerSync(ctx context.Context, req *controlv1.TriggerSyncRequest) (*controlv1.TriggerSyncResponse, error) {
	if req.GetTenantId() == "" || req.GetWindowFrom() == nil || req.GetWindowTo() == nil {
		return nil, status.Error(codes.InvalidArgument, "tenant_id, window_from and window_to are required")
	}
	w := domain.Window{From: req.GetWindowFrom().AsTime(), To: req.GetWindowTo().AsTime()}
	if !w.From.Before(w.To) {
		return nil, status.Error(codes.InvalidArgument, "window_from must be before window_to")
	}
	job := domain.SyncJob{
		JobID:     newID(),
		TenantID:  req.GetTenantId(),
		Source:    orDefault(req.GetSource(), "fake-market"),
		Resource:  orDefault(req.GetResource(), "orders"),
		Window:    w,
		CreatedAt: time.Now().UTC(),
	}
	if err := s.queue.Enqueue(ctx, job); err != nil {
		return nil, status.Errorf(codes.Unavailable, "enqueue: %v", err)
	}
	return &controlv1.TriggerSyncResponse{JobId: job.JobID}, nil
}

func (s *server) GetCursor(ctx context.Context, req *controlv1.GetCursorRequest) (*controlv1.GetCursorResponse, error) {
	c, found, err := s.cursors.Get(ctx, req.GetTenantId(), orDefault(req.GetSource(), "fake-market"), orDefault(req.GetResource(), "orders"))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get cursor: %v", err)
	}
	if !found {
		return nil, status.Error(codes.NotFound, "no cursor yet")
	}
	return &controlv1.GetCursorResponse{
		Watermark: timestamppb.New(c.Watermark),
		UpdatedAt: timestamppb.New(c.UpdatedAt),
		Version:   c.Version,
	}, nil
}

// logging is a unary interceptor that logs method, code and latency.
func logging(log *slog.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		start := time.Now()
		resp, err := handler(ctx, req)
		log.Info("rpc", "method", info.FullMethod, "code", status.Code(err).String(), "duration_ms", time.Since(start).Milliseconds())
		return resp, err
	}
}

func main() {
	log := platform.NewLogger("control")
	ctx, stop := platform.SignalContext()
	defer stop()

	awsCfg, err := platform.AWSConfig(ctx)
	if err != nil {
		log.Error("aws config", "err", err)
		os.Exit(1)
	}

	gs := grpc.NewServer(grpc.ChainUnaryInterceptor(logging(log)))
	controlv1.RegisterControlServiceServer(gs, &server{
		queue:   sqsqueue.New(sqs.NewFromConfig(awsCfg), platform.Env("SYNC_QUEUE_URL", "http://localhost:9324/000000000000/sync-jobs")),
		cursors: dynamostore.NewCursorStore(dynamodb.NewFromConfig(awsCfg), platform.Env("CURSOR_TABLE", "sync_cursors")),
	})
	healthpb.RegisterHealthServer(gs, health.NewServer())

	addr := platform.Env("CONTROL_ADDR", ":9090")
	lis, err := (&net.ListenConfig{}).Listen(ctx, "tcp", addr)
	if err != nil {
		log.Error("listen", "err", err)
		os.Exit(1)
	}

	go func() {
		<-ctx.Done()
		log.Info("shutting down grpc server")
		gs.GracefulStop()
	}()
	log.Info("grpc server listening", "addr", addr)
	if err := gs.Serve(lis); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
		log.Error("serve", "err", err)
		os.Exit(1)
	}
}

func newID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func orDefault(v, def string) string {
	if v == "" {
		return def
	}
	return v
}
