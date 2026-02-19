package domain

import "time"

type Notification struct {
	NotificationID string    `json:"id" dynamodbav:"notification_id"`
	UserID         string    `json:"user_id" dynamodbav:"user_id"`
	DeviceID       *string   `json:"device_id" dynamodbav:"device_id"`
	TemplateID     *string   `json:"template_id" dynamodbav:"template_id"`
	Message        string    `json:"message" dynamodbav:"message"`
	Readed         int       `json:"readed" dynamodbav:"readed"` // legacy field name preserved
	CreatedAt      time.Time `json:"created" dynamodbav:"created_at"`
	UpdatedAt      time.Time `json:"updated" dynamodbav:"updated_at"`
}
