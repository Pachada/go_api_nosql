package dynamo

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/go-api-nosql/internal/config"
)

// NewClient creates a DynamoDB client. When cfg.AWSEndpointURL is set (LocalStack),
// it overrides the endpoint so all traffic goes to the local instance.
func NewClient(cfg *config.Config) *dynamodb.Client {
	opts := []func(*awsconfig.LoadOptions) error{
		awsconfig.WithRegion(cfg.AWSRegion),
	}

	if cfg.AWSAccessKeyID != "" {
		opts = append(opts, awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(cfg.AWSAccessKeyID, cfg.AWSSecretKey, ""),
		))
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(context.Background(), opts...)
	if err != nil {
		panic("failed to load AWS config: " + err.Error())
	}

	clientOpts := []func(*dynamodb.Options){}
	if cfg.AWSEndpointURL != "" {
		clientOpts = append(clientOpts, func(o *dynamodb.Options) {
			o.BaseEndpoint = aws.String(cfg.AWSEndpointURL)
		})
	}

	return dynamodb.NewFromConfig(awsCfg, clientOpts...)
}
