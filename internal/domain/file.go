package domain

import "time"

type File struct {
	FileID            string    `json:"id" dynamodbav:"file_id"`
	Object            string    `json:"object" dynamodbav:"object"`
	Size              int64     `json:"size" dynamodbav:"size"`
	Type              string    `json:"type" dynamodbav:"type"`
	Name              string    `json:"name" dynamodbav:"name"`
	Hash              string    `json:"hash" dynamodbav:"hash"`
	IsThumbnail       int       `json:"is_thumbnail" dynamodbav:"is_thumbnail"`
	URL               *string   `json:"url" dynamodbav:"url"`
	IsPrivate         bool      `json:"is_private" dynamodbav:"is_private"`
	UploadedByUserID  string    `json:"user_who_uploaded_id" dynamodbav:"uploaded_by_user_id"`
	Enable            bool      `json:"enable" dynamodbav:"enable"`
	CreatedAt         time.Time `json:"created" dynamodbav:"created_at"`
	UpdatedAt         time.Time `json:"updated" dynamodbav:"updated_at"`
}
