package dynamo

import (
	"context"
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/go-api-nosql/internal/domain"
)

// RoleRepo provides typed DynamoDB operations for the roles table.
type RoleRepo struct {
	client    *dynamodb.Client
	tableName string
}

func NewRoleRepo(client *dynamodb.Client, tableName string) *RoleRepo {
	return &RoleRepo{client: client, tableName: tableName}
}

func (r *RoleRepo) Put(ctx context.Context, role *domain.Role) error {
	item, err := attributevalue.MarshalMap(role)
	if err != nil {
		return fmt.Errorf("marshal role: %w", err)
	}
	_, err = r.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(r.tableName),
		Item:      item,
	})
	return err
}

func (r *RoleRepo) Get(ctx context.Context, roleID string) (*domain.Role, error) {
	out, err := r.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(r.tableName),
		Key:       strKey("role_id", roleID),
	})
	if err != nil {
		return nil, err
	}
	if out.Item == nil {
		return nil, errors.New("role not found")
	}
	var role domain.Role
	if err := attributevalue.UnmarshalMap(out.Item, &role); err != nil {
		return nil, err
	}
	return &role, nil
}

func (r *RoleRepo) Scan(ctx context.Context) ([]domain.Role, error) {
	out, err := r.client.Scan(ctx, &dynamodb.ScanInput{TableName: aws.String(r.tableName)})
	if err != nil {
		return nil, err
	}
	var roles []domain.Role
	if err := attributevalue.UnmarshalListOfMaps(out.Items, &roles); err != nil {
		return nil, err
	}
	return roles, nil
}

func (r *RoleRepo) Delete(ctx context.Context, roleID string) error {
	_, err := r.client.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: aws.String(r.tableName),
		Key:       strKey("role_id", roleID),
	})
	return err
}

func (r *RoleRepo) Update(ctx context.Context, roleID string, updates map[string]interface{}) error {
	expr, names, values, err := buildUpdateExpr(updates)
	if err != nil {
		return err
	}
	_, err = r.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName:                 aws.String(r.tableName),
		Key:                       strKey("role_id", roleID),
		UpdateExpression:          aws.String(expr),
		ExpressionAttributeNames:  names,
		ExpressionAttributeValues: values,
	})
	return err
}
