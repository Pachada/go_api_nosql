package dynamo

import (
	"context"
	"fmt"
	"log/slog"
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
		return nil, fmt.Errorf("session not found: %w", domain.ErrNotFound)
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
	var firstErr error
	for _, item := range out.Items {
		sidAttr, ok := item["session_id"].(*types.AttributeValueMemberS)
		if !ok {
			continue
		}
		if err := r.Update(ctx, sidAttr.Value, map[string]interface{}{fieldEnable: false}); err != nil {
			slog.Warn("failed to disable session during user soft-delete", "session_id", sidAttr.Value, "user_id", userID, "err", err)
			if firstErr == nil {
				firstErr = err
			}
		}
	}
	return firstErr
}

func (r *SessionRepo) Update(ctx context.Context, sessionID string, updates map[string]interface{}) error {
	updates["updated_at"] = time.Now().UTC().Format(time.RFC3339)
	ue, err := buildUpdateExpr(updates)
	if err != nil {
		return err
	}
	_, err = r.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName:                 aws.String(r.tableName),
		Key:                       strKey("session_id", sessionID),
		UpdateExpression:          aws.String(ue.Expr),
		ExpressionAttributeNames:  ue.Names,
		ExpressionAttributeValues: ue.Values,
	})
	return err
}

// GetByRefreshToken looks up a session by its opaque refresh token via GSI.
// Returns ErrUnauthorized (session disabled) when found but inactive.
func (r *SessionRepo) GetByRefreshToken(ctx context.Context, token string) (*domain.Session, error) {
	out, err := r.client.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(r.tableName),
		IndexName:              aws.String("refresh_token-index"),
		KeyConditionExpression: aws.String("refresh_token = :rt"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":rt": &types.AttributeValueMemberS{Value: token},
		},
	})
	if err != nil {
		return nil, err
	}
	if len(out.Items) == 0 {
		return nil, fmt.Errorf("session not found: %w", domain.ErrNotFound)
	}
	var s domain.Session
	if err := attributevalue.UnmarshalMap(out.Items[0], &s); err != nil {
		return nil, err
	}
	if !s.Enable {
		return nil, fmt.Errorf("session disabled: %w", domain.ErrUnauthorized)
	}
	return &s, nil
}

// RotateRefreshToken replaces the refresh token and expiry on a session.
func (r *SessionRepo) RotateRefreshToken(ctx context.Context, sessionID, newToken string, newExpiry int64) error {
	return r.Update(ctx, sessionID, map[string]interface{}{
		fieldRefreshToken:     newToken,
		fieldRefreshExpiresAt: newExpiry,
	})
}
