package dynamo

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/go-api-nosql/internal/domain"
)

// FileRepo provides typed DynamoDB operations for the files table.
type FileRepo struct {
	client    *dynamodb.Client
	tableName string
}

func NewFileRepo(client *dynamodb.Client, tableName string) *FileRepo {
	return &FileRepo{client: client, tableName: tableName}
}

func (r *FileRepo) Put(ctx context.Context, f *domain.File) error {
	item, err := attributevalue.MarshalMap(f)
	if err != nil {
		return fmt.Errorf("marshal file: %w", err)
	}
	_, err = r.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(r.tableName),
		Item:      item,
	})
	return err
}

func (r *FileRepo) Get(ctx context.Context, fileID string) (*domain.File, error) {
	out, err := r.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(r.tableName),
		Key:       strKey("file_id", fileID),
	})
	if err != nil {
		return nil, err
	}
	if out.Item == nil {
		return nil, fmt.Errorf("file not found: %w", domain.ErrNotFound)
	}
	var f domain.File
	if err := attributevalue.UnmarshalMap(out.Item, &f); err != nil {
		return nil, err
	}
	return &f, nil
}

func (r *FileRepo) SoftDelete(ctx context.Context, fileID string) error {
	return r.update(ctx, fileID, map[string]interface{}{fieldEnable: false})
}

func (r *FileRepo) update(ctx context.Context, fileID string, updates map[string]interface{}) error {
	updates["updated_at"] = time.Now().UTC().Format(time.RFC3339)
	ue, err := buildUpdateExpr(updates)
	if err != nil {
		return err
	}
	_, err = r.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName:                 aws.String(r.tableName),
		Key:                       strKey("file_id", fileID),
		UpdateExpression:          aws.String(ue.Expr),
		ExpressionAttributeNames:  ue.Names,
		ExpressionAttributeValues: ue.Values,
	})
	return err
}
