// Package sqsqueue implements ports.Queue on Amazon SQS. Locally it runs
// against ElasticMQ, which speaks the SQS API.
package sqsqueue

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"

	"github.com/ngocmx2104/market-sync/internal/domain"
	"github.com/ngocmx2104/market-sync/internal/ports"
)

type Queue struct {
	client   *sqs.Client
	queueURL string
}

var _ ports.Queue = (*Queue)(nil)

func New(client *sqs.Client, queueURL string) *Queue {
	return &Queue{client: client, queueURL: queueURL}
}

func (q *Queue) Enqueue(ctx context.Context, jobs ...domain.SyncJob) error {
	// SendMessageBatch accepts at most 10 entries.
	for start := 0; start < len(jobs); start += 10 {
		end := min(start+10, len(jobs))
		entries := make([]types.SendMessageBatchRequestEntry, 0, end-start)
		for i, job := range jobs[start:end] {
			body, err := json.Marshal(job)
			if err != nil {
				return err
			}
			entries = append(entries, types.SendMessageBatchRequestEntry{
				Id:          aws.String(strconv.Itoa(i)),
				MessageBody: aws.String(string(body)),
			})
		}
		out, err := q.client.SendMessageBatch(ctx, &sqs.SendMessageBatchInput{QueueUrl: &q.queueURL, Entries: entries})
		if err != nil {
			return fmt.Errorf("sqs send batch: %w", err)
		}
		if len(out.Failed) > 0 {
			return fmt.Errorf("sqs send batch: %d of %d entries failed, first: %s", len(out.Failed), len(entries), aws.ToString(out.Failed[0].Message))
		}
	}
	return nil
}

func (q *Queue) Receive(ctx context.Context, maxMsgs int, wait time.Duration) ([]ports.Message, error) {
	out, err := q.client.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
		QueueUrl:                    &q.queueURL,
		MaxNumberOfMessages:         clamp32(int64(maxMsgs), 1, 10),
		WaitTimeSeconds:             clamp32(int64(wait/time.Second), 0, 20),
		MessageSystemAttributeNames: []types.MessageSystemAttributeName{types.MessageSystemAttributeNameApproximateReceiveCount},
	})
	if err != nil {
		return nil, fmt.Errorf("sqs receive: %w", err)
	}

	msgs := make([]ports.Message, 0, len(out.Messages))
	for _, m := range out.Messages {
		var job domain.SyncJob
		if err := json.Unmarshal([]byte(aws.ToString(m.Body)), &job); err != nil {
			// A malformed body can never succeed; leave it for the redrive
			// policy to move to the DLQ.
			continue
		}
		count, _ := strconv.Atoi(m.Attributes[string(types.MessageSystemAttributeNameApproximateReceiveCount)])
		msgs = append(msgs, ports.Message{Job: job, ReceiptHandle: aws.ToString(m.ReceiptHandle), ReceiveCount: count})
	}
	return msgs, nil
}

func (q *Queue) Ack(ctx context.Context, m ports.Message) error {
	_, err := q.client.DeleteMessage(ctx, &sqs.DeleteMessageInput{QueueUrl: &q.queueURL, ReceiptHandle: &m.ReceiptHandle})
	return err
}

func (q *Queue) ExtendLease(ctx context.Context, m ports.Message, d time.Duration) error {
	_, err := q.client.ChangeMessageVisibility(ctx, &sqs.ChangeMessageVisibilityInput{
		QueueUrl:          &q.queueURL,
		ReceiptHandle:     &m.ReceiptHandle,
		VisibilityTimeout: clamp32(int64(d/time.Second), 0, 12*60*60),
	})
	return err
}

// clamp32 bounds v to [lo, hi] (SQS limits) before converting to int32.
func clamp32(v, lo, hi int64) int32 {
	return int32(min(max(v, lo), hi)) //nolint:gosec // bounded by lo/hi, which fit in int32
}
