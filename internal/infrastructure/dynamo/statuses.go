package dynamo

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/go-api-nosql/internal/domain"
)

// StatusRepo provides typed DynamoDB operations for the statuses table.
type StatusRepo struct {
	client    *dynamodb.Client
	tableName string
}

func NewStatusRepo(client *dynamodb.Client, tableName string) *StatusRepo {
	return &StatusRepo{client: client, tableName: tableName}
}

func (r *StatusRepo) Put(ctx context.Context, s *domain.Status) error {
	item, err := attributevalue.MarshalMap(s)
	if err != nil {
		return fmt.Errorf("marshal status: %w", err)
	}
	_, err = r.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(r.tableName),
		Item:      item,
	})
	return err
}

func (r *StatusRepo) Get(ctx context.Context, statusID string) (*domain.Status, error) {
	out, err := r.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(r.tableName),
		Key:       strKey("status_id", statusID),
	})
	if err != nil {
		return nil, err
	}
	if out.Item == nil {
		return nil, fmt.Errorf("status not found: %w", domain.ErrNotFound)
	}
	var s domain.Status
	if err := attributevalue.UnmarshalMap(out.Item, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *StatusRepo) Scan(ctx context.Context) ([]domain.Status, error) {
	out, err := r.client.Scan(ctx, &dynamodb.ScanInput{TableName: aws.String(r.tableName)})
	if err != nil {
		return nil, err
	}
	var statuses []domain.Status
	if err := attributevalue.UnmarshalListOfMaps(out.Items, &statuses); err != nil {
		return nil, err
	}
	return statuses, nil
}

// HardDelete permanently removes a status item (no soft delete for statuses).
func (r *StatusRepo) HardDelete(ctx context.Context, statusID string) error {
	_, err := r.client.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: aws.String(r.tableName),
		Key:       strKey("status_id", statusID),
	})
	return err
}

func (r *StatusRepo) Update(ctx context.Context, statusID string, updates map[string]interface{}) error {
	ue, err := buildUpdateExpr(updates)
	if err != nil {
		return err
	}
	_, err = r.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName:                 aws.String(r.tableName),
		Key:                       strKey("status_id", statusID),
		UpdateExpression:          aws.String(ue.Expr),
		ExpressionAttributeNames:  ue.Names,
		ExpressionAttributeValues: ue.Values,
	})
	return err
}
