package sns

import (
	"context"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/go-api-nosql/internal/config"
)

// SMSSender sends SMS messages via AWS SNS.
type SMSSender interface {
	SendSMS(ctx context.Context, to, message string) error
}

type sender struct {
	client *sns.Client
}

func NewSender(cfg *config.Config) (SMSSender, error) {
	awsCfg, err := awsconfig.LoadDefaultConfig(context.Background(),
		awsconfig.WithRegion(cfg.SNSRegion),
	)
	if err != nil {
		return nil, err
	}
	return &sender{client: sns.NewFromConfig(awsCfg)}, nil
}

func (s *sender) SendSMS(ctx context.Context, to, message string) error {
	_, err := s.client.Publish(ctx, &sns.PublishInput{
		PhoneNumber: &to,
		Message:     &message,
	})
	return err
}
