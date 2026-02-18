package dynamo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/go-api-nosql/internal/domain"
)

// SessionRepo provides typed DynamoDB operations for the sessions table.
type SessionRepo struct {
	client    *dynamodb.Client
	tableName string
}

func NewSessionRepo(client *dynamodb.Client, tableName string) *SessionRepo {
	return &SessionRepo{client: client, tableName: tableName}
}

func (r *SessionRepo) Put(ctx context.Context, s *domain.Session) error {
	item, err := attributevalue.MarshalMap(s)
	if err != nil {
		return fmt.Errorf("marshal session: %w", err)
	}
	_, err = r.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(r.tableName),
		Item:      item,
	})
	return err
}

func (r *SessionRepo) Get(ctx context.Context, sessionID string) (*domain.Session, error) {
	out, err := r.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(r.tableName),
		Key:       strKey("session_id", sessionID),
	})
	if err != nil {
		return nil, err
	}
	if out.Item == nil {
		return nil, errors.New("session not found")
	}
	var s domain.Session
	if err := attributevalue.UnmarshalMap(out.Item, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *SessionRepo) SoftDeleteByUser(ctx context.Context, userID string) error {
	out, err := r.client.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(r.tableName),
		IndexName:              aws.String("user_id-index"),
		KeyConditionExpression: aws.String("user_id = :uid"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":uid": &types.AttributeValueMemberS{Value: userID},
		},
	})
	if err != nil {
		return err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	for _, item := range out.Items {
		sidAttr, ok := item["session_id"].(*types.AttributeValueMemberS)
		if !ok {
			continue
		}
		_ = r.Update(ctx, sidAttr.Value, map[string]interface{}{"enable": false, "updated_at": now})
	}
	return nil
}

func (r *SessionRepo) Update(ctx context.Context, sessionID string, updates map[string]interface{}) error {
	updates["updated_at"] = time.Now().UTC().Format(time.RFC3339)
	expr, names, values, err := buildUpdateExpr(updates)
	if err != nil {
		return err
	}
	_, err = r.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName:                 aws.String(r.tableName),
		Key:                       strKey("session_id", sessionID),
		UpdateExpression:          aws.String(expr),
		ExpressionAttributeNames:  names,
		ExpressionAttributeValues: values,
	})
	return err
}
