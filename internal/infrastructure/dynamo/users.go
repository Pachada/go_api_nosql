package dynamo

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/go-api-nosql/internal/domain"
)

// UserRepo provides typed DynamoDB operations for the users table.
type UserRepo struct {
	client    *dynamodb.Client
	tableName string
}

func NewUserRepo(client *dynamodb.Client, tableName string) *UserRepo {
	return &UserRepo{client: client, tableName: tableName}
}

func (r *UserRepo) Put(ctx context.Context, u *domain.User) error {
	item, err := attributevalue.MarshalMap(u)
	if err != nil {
		return fmt.Errorf("marshal user: %w", err)
	}
	_, err = r.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(r.tableName),
		Item:      item,
	})
	return err
}

func (r *UserRepo) Get(ctx context.Context, userID string) (*domain.User, error) {
	out, err := r.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(r.tableName),
		Key:       strKey("user_id", userID),
	})
	if err != nil {
		return nil, err
	}
	if out.Item == nil {
		return nil, errors.New("user not found")
	}
	var u domain.User
	if err := attributevalue.UnmarshalMap(out.Item, &u); err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *UserRepo) GetByUsername(ctx context.Context, username string) (*domain.User, error) {
	return r.queryGSI(ctx, "username-index", "username", username)
}

func (r *UserRepo) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	return r.queryGSI(ctx, "email-index", "email", email)
}

func (r *UserRepo) Update(ctx context.Context, userID string, updates map[string]interface{}) error {
	updates["updated_at"] = time.Now().UTC().Format(time.RFC3339)
	expr, names, values, err := buildUpdateExpr(updates)
	if err != nil {
		return err
	}
	_, err = r.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName:                 aws.String(r.tableName),
		Key:                       strKey("user_id", userID),
		UpdateExpression:          aws.String(expr),
		ExpressionAttributeNames:  names,
		ExpressionAttributeValues: values,
	})
	return err
}

func (r *UserRepo) SoftDelete(ctx context.Context, userID string) error {
	return r.Update(ctx, userID, map[string]interface{}{"enable": false})
}

// ScanPage returns a page of enabled users.
// cursor is a base64-encoded user_id used as ExclusiveStartKey.
// Returns the items, a next cursor (empty string when no more pages), and any error.
func (r *UserRepo) ScanPage(ctx context.Context, limit int32, cursor string) ([]domain.User, string, error) {
	input := &dynamodb.ScanInput{
		TableName:        aws.String(r.tableName),
		FilterExpression: aws.String("enable = :t"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":t": &types.AttributeValueMemberBOOL{Value: true},
		},
		Limit: aws.Int32(limit),
	}
	if cursor != "" {
		userID, err := decodeCursor(cursor)
		if err != nil {
			return nil, "", fmt.Errorf("invalid cursor: %w", domain.ErrBadRequest)
		}
		input.ExclusiveStartKey = map[string]types.AttributeValue{
			"user_id": &types.AttributeValueMemberS{Value: userID},
		}
	}
	out, err := r.client.Scan(ctx, input)
	if err != nil {
		return nil, "", err
	}
	var users []domain.User
	if err := attributevalue.UnmarshalListOfMaps(out.Items, &users); err != nil {
		return nil, "", err
	}
	nextCursor := ""
	if v, ok := out.LastEvaluatedKey["user_id"].(*types.AttributeValueMemberS); ok {
		nextCursor = encodeCursor(v.Value)
	}
	return users, nextCursor, nil
}

func encodeCursor(userID string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(userID))
}

func decodeCursor(cursor string) (string, error) {
	b, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func (r *UserRepo) queryGSI(ctx context.Context, index, attr, value string) (*domain.User, error) {
	out, err := r.client.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(r.tableName),
		IndexName:              aws.String(index),
		KeyConditionExpression: aws.String("#a = :v"),
		ExpressionAttributeNames:  map[string]string{"#a": attr},
		ExpressionAttributeValues: map[string]types.AttributeValue{":v": &types.AttributeValueMemberS{Value: value}},
		Limit:                  aws.Int32(1),
	})
	if err != nil {
		return nil, err
	}
	if len(out.Items) == 0 {
		return nil, errors.New("user not found")
	}
	var u domain.User
	if err := attributevalue.UnmarshalMap(out.Items[0], &u); err != nil {
		return nil, err
	}
	return &u, nil
}
