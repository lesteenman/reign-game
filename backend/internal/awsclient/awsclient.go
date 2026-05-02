// Package awsclient centralizes AWS SDK setup for the project's
// Lambda binaries (cmd/api, cmd/daily-cron, future cron Lambdas).
//
// Two exported functions:
//
//   - LoadAWSConfig — loads the default AWS SDK config with the
//     project's required customizations: a region from AWS_REGION
//     env (default us-east-1), and an isolated http.Transport
//     (clone of http.DefaultTransport) so other SDKs in the same
//     binary cannot mutate this client's underlying transport.
//     The transport-isolation is from CLAUDE.md lesson 24 (Clerk's
//     Go SDK mutates http.DefaultTransport, which adds ~1.8s to
//     the first DDB request via the shared default).
//
//   - NewDynamoDBClient — constructs a DynamoDB client that honors
//     the optional DYNAMODB_ENDPOINT env (LocalStack support for
//     local dev + integration tests).
//
// Each Lambda's main() is responsible for selecting the AWS service
// clients it needs from the loaded config; this package handles
// the bootstrap that's identical across binaries.
package awsclient

import (
	"context"
	"fmt"
	"net/http"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
)

// LoadAWSConfig is documented at the package level.
func LoadAWSConfig(ctx context.Context) (aws.Config, error) {
	region := os.Getenv("AWS_REGION")
	if region == "" {
		region = "us-east-1"
	}

	httpClient := &http.Client{
		Transport: http.DefaultTransport.(*http.Transport).Clone(),
	}

	cfg, err := config.LoadDefaultConfig(
		ctx,
		config.WithRegion(region),
		config.WithHTTPClient(httpClient),
	)
	if err != nil {
		return aws.Config{}, fmt.Errorf("loading AWS config: %w", err)
	}
	return cfg, nil
}

// NewDynamoDBClient is documented at the package level.
func NewDynamoDBClient(cfg aws.Config) *dynamodb.Client {
	if endpoint := os.Getenv("DYNAMODB_ENDPOINT"); endpoint != "" {
		return dynamodb.NewFromConfig(cfg, func(o *dynamodb.Options) {
			o.BaseEndpoint = aws.String(endpoint)
		})
	}
	return dynamodb.NewFromConfig(cfg)
}
