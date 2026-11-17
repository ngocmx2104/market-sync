package platform

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
)

// AWSConfig loads the default AWS config. For local runs, point the SDK at
// ElasticMQ / DynamoDB Local with AWS_ENDPOINT_URL_SQS and
// AWS_ENDPOINT_URL_DYNAMODB (supported natively by aws-sdk-go-v2).
func AWSConfig(ctx context.Context) (aws.Config, error) {
	return config.LoadDefaultConfig(ctx, config.WithRegion(Env("AWS_REGION", "ap-southeast-1")))
}
