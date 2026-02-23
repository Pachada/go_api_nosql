package dynamo

import (
	"context"
	"errors"
	"log/slog"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/go-api-nosql/internal/config"
)

// Bootstrap creates all DynamoDB tables and GSIs if they don't already exist.
// Safe to call on every startup — skips tables that already exist.
func Bootstrap(ctx context.Context, client *dynamodb.Client, tables config.DynamoTables) {
	createTable(ctx, client, &dynamodb.CreateTableInput{
		TableName:   aws.String(tables.Users),
		BillingMode: types.BillingModePayPerRequest,
		AttributeDefinitions: []types.AttributeDefinition{
			{AttributeName: aws.String("user_id"), AttributeType: types.ScalarAttributeTypeS},
			{AttributeName: aws.String("username"), AttributeType: types.ScalarAttributeTypeS},
			{AttributeName: aws.String("email"), AttributeType: types.ScalarAttributeTypeS},
			{AttributeName: aws.String("enable"), AttributeType: types.ScalarAttributeTypeN},
		},
		KeySchema: []types.KeySchemaElement{
			{AttributeName: aws.String("user_id"), KeyType: types.KeyTypeHash},
		},
		GlobalSecondaryIndexes: []types.GlobalSecondaryIndex{
			gsi("username-index", "username", ""),
			gsi("email-index", "email", ""),
			gsi("enable-index", "enable", ""),
		},
	})

	createTable(ctx, client, &dynamodb.CreateTableInput{
		TableName:   aws.String(tables.Sessions),
		BillingMode: types.BillingModePayPerRequest,
		AttributeDefinitions: []types.AttributeDefinition{
			{AttributeName: aws.String("session_id"), AttributeType: types.ScalarAttributeTypeS},
			{AttributeName: aws.String("user_id"), AttributeType: types.ScalarAttributeTypeS},
			{AttributeName: aws.String("refresh_token"), AttributeType: types.ScalarAttributeTypeS},
		},
		KeySchema: []types.KeySchemaElement{
			{AttributeName: aws.String("session_id"), KeyType: types.KeyTypeHash},
		},
		GlobalSecondaryIndexes: []types.GlobalSecondaryIndex{
			gsi("user_id-index", "user_id", ""),
			gsi("refresh_token-index", "refresh_token", ""),
		},
	})

	createTable(ctx, client, &dynamodb.CreateTableInput{
		TableName:   aws.String(tables.Statuses),
		BillingMode: types.BillingModePayPerRequest,
		AttributeDefinitions: []types.AttributeDefinition{
			{AttributeName: aws.String("status_id"), AttributeType: types.ScalarAttributeTypeS},
		},
		KeySchema: []types.KeySchemaElement{
			{AttributeName: aws.String("status_id"), KeyType: types.KeyTypeHash},
		},
	})

	createTable(ctx, client, &dynamodb.CreateTableInput{
		TableName:   aws.String(tables.Devices),
		BillingMode: types.BillingModePayPerRequest,
		AttributeDefinitions: []types.AttributeDefinition{
			{AttributeName: aws.String("device_id"), AttributeType: types.ScalarAttributeTypeS},
			{AttributeName: aws.String("user_id"), AttributeType: types.ScalarAttributeTypeS},
			{AttributeName: aws.String("device_uuid"), AttributeType: types.ScalarAttributeTypeS},
		},
		KeySchema: []types.KeySchemaElement{
			{AttributeName: aws.String("device_id"), KeyType: types.KeyTypeHash},
		},
		GlobalSecondaryIndexes: []types.GlobalSecondaryIndex{
			gsi("user_id-index", "user_id", ""),
			gsi("device_uuid-index", "device_uuid", ""),
		},
	})

	createTable(ctx, client, &dynamodb.CreateTableInput{
		TableName:   aws.String(tables.Notifications),
		BillingMode: types.BillingModePayPerRequest,
		AttributeDefinitions: []types.AttributeDefinition{
			{AttributeName: aws.String("notification_id"), AttributeType: types.ScalarAttributeTypeS},
			{AttributeName: aws.String("user_id"), AttributeType: types.ScalarAttributeTypeS},
			{AttributeName: aws.String("created_at"), AttributeType: types.ScalarAttributeTypeS},
		},
		KeySchema: []types.KeySchemaElement{
			{AttributeName: aws.String("notification_id"), KeyType: types.KeyTypeHash},
		},
		GlobalSecondaryIndexes: []types.GlobalSecondaryIndex{
			gsi("user_id-created_at-index", "user_id", "created_at"),
		},
	})

	createTable(ctx, client, &dynamodb.CreateTableInput{
		TableName:   aws.String(tables.Files),
		BillingMode: types.BillingModePayPerRequest,
		AttributeDefinitions: []types.AttributeDefinition{
			{AttributeName: aws.String("file_id"), AttributeType: types.ScalarAttributeTypeS},
			{AttributeName: aws.String("uploaded_by_user_id"), AttributeType: types.ScalarAttributeTypeS},
		},
		KeySchema: []types.KeySchemaElement{
			{AttributeName: aws.String("file_id"), KeyType: types.KeyTypeHash},
		},
		GlobalSecondaryIndexes: []types.GlobalSecondaryIndex{
			gsi("uploaded_by_user_id-index", "uploaded_by_user_id", ""),
		},
	})

	createTable(ctx, client, &dynamodb.CreateTableInput{
		TableName:   aws.String(tables.UserVerifications),
		BillingMode: types.BillingModePayPerRequest,
		AttributeDefinitions: []types.AttributeDefinition{
			{AttributeName: aws.String("user_id"), AttributeType: types.ScalarAttributeTypeS},
			{AttributeName: aws.String("type"), AttributeType: types.ScalarAttributeTypeS},
		},
		KeySchema: []types.KeySchemaElement{
			{AttributeName: aws.String("user_id"), KeyType: types.KeyTypeHash},
			{AttributeName: aws.String("type"), KeyType: types.KeyTypeRange},
		},
	})
	enableTTL(ctx, client, tables.UserVerifications, "expires_at")

	createTable(ctx, client, &dynamodb.CreateTableInput{
		TableName:   aws.String(tables.AppVersions),
		BillingMode: types.BillingModePayPerRequest,
		AttributeDefinitions: []types.AttributeDefinition{
			{AttributeName: aws.String("version_id"), AttributeType: types.ScalarAttributeTypeS},
		},
		KeySchema: []types.KeySchemaElement{
			{AttributeName: aws.String("version_id"), KeyType: types.KeyTypeHash},
		},
	})
}

// gsi builds a GSI descriptor. If sortKey is empty, only a hash key is added.
func gsi(indexName, hashKey, sortKey string) types.GlobalSecondaryIndex {
	ks := []types.KeySchemaElement{
		{AttributeName: aws.String(hashKey), KeyType: types.KeyTypeHash},
	}
	if sortKey != "" {
		ks = append(ks, types.KeySchemaElement{
			AttributeName: aws.String(sortKey), KeyType: types.KeyTypeRange,
		})
	}
	return types.GlobalSecondaryIndex{
		IndexName:  aws.String(indexName),
		KeySchema:  ks,
		Projection: &types.Projection{ProjectionType: types.ProjectionTypeAll},
	}
}

func createTable(ctx context.Context, client *dynamodb.Client, input *dynamodb.CreateTableInput) {
	_, err := client.CreateTable(ctx, input)
	if err != nil {
		// ResourceInUseException means the table already exists — that's fine.
		var riue *types.ResourceInUseException
		if !errors.As(err, &riue) {
			slog.Warn("could not create table", "table", *input.TableName, "err", err)
		}
	} else {
		slog.Info("created table", "table", *input.TableName)
	}
}

func enableTTL(ctx context.Context, client *dynamodb.Client, tableName, ttlAttr string) {
	_, err := client.UpdateTimeToLive(ctx, &dynamodb.UpdateTimeToLiveInput{
		TableName: aws.String(tableName),
		TimeToLiveSpecification: &types.TimeToLiveSpecification{
			Enabled:       aws.Bool(true),
			AttributeName: aws.String(ttlAttr),
		},
	})
	if err != nil {
		slog.Warn("could not enable TTL", "table", tableName, "err", err)
	}
}
