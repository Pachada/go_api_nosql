package domain

import "time"

type UpdateDeviceRequest struct {
	Token        *string `json:"token"`
	AppVersionID *string `json:"app_version_id"`
}

type Device struct {
	DeviceID     string    `json:"id" dynamodbav:"device_id"`
	UUID         string    `json:"uuid" dynamodbav:"device_uuid"`
	UserID       string    `json:"user_id" dynamodbav:"user_id"`
	Token        *string   `json:"token" dynamodbav:"token"`
	AppVersionID string    `json:"app_version_id" dynamodbav:"app_version_id"`
	Enable       bool      `json:"enable" dynamodbav:"enable"`
	CreatedAt    time.Time `json:"created" dynamodbav:"created_at"`
	UpdatedAt    time.Time `json:"updated" dynamodbav:"updated_at"`
}
