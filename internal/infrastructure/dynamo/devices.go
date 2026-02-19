package dynamo

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/go-api-nosql/internal/domain"
)

// DeviceRepo provides typed DynamoDB operations for the devices table.
type DeviceRepo struct {
	client    *dynamodb.Client
	tableName string
}

func NewDeviceRepo(client *dynamodb.Client, tableName string) *DeviceRepo {
	return &DeviceRepo{client: client, tableName: tableName}
}

func (r *DeviceRepo) Put(ctx context.Context, d *domain.Device) error {
	item, err := attributevalue.MarshalMap(d)
	if err != nil {
		return fmt.Errorf("marshal device: %w", err)
	}
	_, err = r.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(r.tableName),
		Item:      item,
	})
	return err
}

func (r *DeviceRepo) Get(ctx context.Context, deviceID string) (*domain.Device, error) {
	out, err := r.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(r.tableName),
		Key:       strKey("device_id", deviceID),
	})
	if err != nil {
		return nil, err
	}
	if out.Item == nil {
		return nil, fmt.Errorf("device not found: %w", domain.ErrNotFound)
	}
	var d domain.Device
	if err := attributevalue.UnmarshalMap(out.Item, &d); err != nil {
		return nil, err
	}
	return &d, nil
}

func (r *DeviceRepo) GetByUUID(ctx context.Context, uuid string) (*domain.Device, error) {
	out, err := r.client.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(r.tableName),
		IndexName:              aws.String("device_uuid-index"),
		KeyConditionExpression: aws.String("device_uuid = :u"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":u": &types.AttributeValueMemberS{Value: uuid},
		},
		Limit: aws.Int32(1),
	})
	if err != nil {
		return nil, err
	}
	if len(out.Items) == 0 {
		return nil, fmt.Errorf("device not found: %w", domain.ErrNotFound)
	}
	var d domain.Device
	if err := attributevalue.UnmarshalMap(out.Items[0], &d); err != nil {
		return nil, err
	}
	return &d, nil
}

func (r *DeviceRepo) ListByUser(ctx context.Context, userID string) ([]domain.Device, error) {
	out, err := r.client.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(r.tableName),
		IndexName:              aws.String("user_id-index"),
		KeyConditionExpression: aws.String("user_id = :uid"),
		FilterExpression:       aws.String("#en = :t"),
		ExpressionAttributeNames: map[string]string{
			"#en": "enable",
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":uid": &types.AttributeValueMemberS{Value: userID},
			":t":   &types.AttributeValueMemberBOOL{Value: true},
		},
	})
	if err != nil {
		return nil, err
	}
	var devices []domain.Device
	if err := attributevalue.UnmarshalListOfMaps(out.Items, &devices); err != nil {
		return nil, err
	}
	return devices, nil
}

func (r *DeviceRepo) Update(ctx context.Context, deviceID string, updates map[string]interface{}) error {
	updates["updated_at"] = time.Now().UTC().Format(time.RFC3339)
	ue, err := buildUpdateExpr(updates)
	if err != nil {
		return err
	}
	_, err = r.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName:                 aws.String(r.tableName),
		Key:                       strKey("device_id", deviceID),
		UpdateExpression:          aws.String(ue.Expr),
		ExpressionAttributeNames:  ue.Names,
		ExpressionAttributeValues: ue.Values,
	})
	return err
}

func (r *DeviceRepo) SoftDelete(ctx context.Context, deviceID string) error {
	return r.Update(ctx, deviceID, map[string]interface{}{"enable": false})
}
