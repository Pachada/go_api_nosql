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

// NotificationRepo provides typed DynamoDB operations for the notifications table.
type NotificationRepo struct {
	client    *dynamodb.Client
	tableName string
}

func NewNotificationRepo(client *dynamodb.Client, tableName string) *NotificationRepo {
	return &NotificationRepo{client: client, tableName: tableName}
}

func (r *NotificationRepo) Put(ctx context.Context, n *domain.Notification) error {
	item, err := attributevalue.MarshalMap(n)
	if err != nil {
		return fmt.Errorf("marshal notification: %w", err)
	}
	_, err = r.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(r.tableName),
		Item:      item,
	})
	return err
}

func (r *NotificationRepo) Get(ctx context.Context, notificationID string) (*domain.Notification, error) {
	out, err := r.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(r.tableName),
		Key:       strKey("notification_id", notificationID),
	})
	if err != nil {
		return nil, err
	}
	if out.Item == nil {
		return nil, errors.New("notification not found")
	}
	var n domain.Notification
	if err := attributevalue.UnmarshalMap(out.Item, &n); err != nil {
		return nil, err
	}
	return &n, nil
}

// ListUnread queries the user_id-created_at GSI and filters for readed=0.
func (r *NotificationRepo) ListUnread(ctx context.Context, userID string) ([]domain.Notification, error) {
	out, err := r.client.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(r.tableName),
		IndexName:              aws.String("user_id-created_at-index"),
		KeyConditionExpression: aws.String("user_id = :uid"),
		FilterExpression:       aws.String("readed = :zero"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":uid":  &types.AttributeValueMemberS{Value: userID},
			":zero": &types.AttributeValueMemberN{Value: "0"},
		},
	})
	if err != nil {
		return nil, err
	}
	var notifications []domain.Notification
	if err := attributevalue.UnmarshalListOfMaps(out.Items, &notifications); err != nil {
		return nil, err
	}
	return notifications, nil
}

func (r *NotificationRepo) MarkAsRead(ctx context.Context, notificationID string) error {
	expr, names, values, err := buildUpdateExpr(map[string]interface{}{"readed": 1})
	if err != nil {
		return err
	}
	_, err = r.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName:                 aws.String(r.tableName),
		Key:                       strKey("notification_id", notificationID),
		UpdateExpression:          aws.String(expr),
		ExpressionAttributeNames:  names,
		ExpressionAttributeValues: values,
	})
	return err
}
