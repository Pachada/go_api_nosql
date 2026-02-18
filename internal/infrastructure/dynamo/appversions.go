package dynamo

import (
	"context"
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/go-api-nosql/internal/domain"
)

// AppVersionRepo provides typed DynamoDB operations for the app_versions table.
type AppVersionRepo struct {
	client    *dynamodb.Client
	tableName string
}

func NewAppVersionRepo(client *dynamodb.Client, tableName string) *AppVersionRepo {
	return &AppVersionRepo{client: client, tableName: tableName}
}

func (r *AppVersionRepo) Put(ctx context.Context, v *domain.AppVersion) error {
	item, err := attributevalue.MarshalMap(v)
	if err != nil {
		return fmt.Errorf("marshal app version: %w", err)
	}
	_, err = r.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(r.tableName),
		Item:      item,
	})
	return err
}

func (r *AppVersionRepo) Get(ctx context.Context, versionID string) (*domain.AppVersion, error) {
	out, err := r.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(r.tableName),
		Key:       strKey("version_id", versionID),
	})
	if err != nil {
		return nil, err
	}
	if out.Item == nil {
		return nil, errors.New("app version not found")
	}
	var v domain.AppVersion
	if err := attributevalue.UnmarshalMap(out.Item, &v); err != nil {
		return nil, err
	}
	return &v, nil
}

// GetLatest returns the most recent enabled app version via full scan (table is tiny).
func (r *AppVersionRepo) GetLatest(ctx context.Context) (*domain.AppVersion, error) {
	out, err := r.client.Scan(ctx, &dynamodb.ScanInput{
		TableName:        aws.String(r.tableName),
		FilterExpression: aws.String("enable = :t"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":t": &types.AttributeValueMemberBOOL{Value: true},
		},
		Limit: aws.Int32(1),
	})
	if err != nil {
		return nil, err
	}
	if len(out.Items) == 0 {
		return nil, errors.New("no active app version found")
	}
	var v domain.AppVersion
	if err := attributevalue.UnmarshalMap(out.Items[0], &v); err != nil {
		return nil, err
	}
	return &v, nil
}
