package dynamo

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/go-api-nosql/internal/domain"
)

// VerificationRepo manages OTP and email verification tokens.
// PK: user_id, SK: type ("otp" | "email")
type VerificationRepo struct {
	client    *dynamodb.Client
	tableName string
}

func NewVerificationRepo(client *dynamodb.Client, tableName string) *VerificationRepo {
	return &VerificationRepo{client: client, tableName: tableName}
}

func (r *VerificationRepo) Put(ctx context.Context, v *domain.UserVerification) error {
	item, err := attributevalue.MarshalMap(v)
	if err != nil {
		return fmt.Errorf("marshal verification: %w", err)
	}
	_, err = r.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(r.tableName),
		Item:      item,
	})
	return err
}

func (r *VerificationRepo) Get(ctx context.Context, userID, verType string) (*domain.UserVerification, error) {
	out, err := r.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(r.tableName),
		Key:       compositeKey("user_id", userID, "type", verType),
	})
	if err != nil {
		return nil, err
	}
	if out.Item == nil {
		return nil, fmt.Errorf("verification not found: %w", domain.ErrNotFound)
	}
	var v domain.UserVerification
	if err := attributevalue.UnmarshalMap(out.Item, &v); err != nil {
		return nil, err
	}
	return &v, nil
}

func (r *VerificationRepo) Delete(ctx context.Context, userID, verType string) error {
	_, err := r.client.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: aws.String(r.tableName),
		Key:       compositeKey("user_id", userID, "type", verType),
	})
	return err
}
