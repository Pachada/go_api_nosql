package domain

import "time"

type Session struct {
	SessionID string    `json:"id" dynamodbav:"session_id"`
	UserID    string    `json:"user_id" dynamodbav:"user_id"`
	DeviceID  string    `json:"device_id" dynamodbav:"device_id"`
	Enable    bool      `json:"enable" dynamodbav:"enable"`
	CreatedAt time.Time `json:"created" dynamodbav:"created_at"`
	UpdatedAt time.Time `json:"updated" dynamodbav:"updated_at"`
	User      *User     `json:"user,omitempty" dynamodbav:"-"`
}
