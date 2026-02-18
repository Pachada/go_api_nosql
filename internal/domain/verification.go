package domain

// UserVerification stores OTP and email confirmation tokens.
// PK: user_id, SK: type ("otp" | "email").
// ExpiresAt is a Unix timestamp used as DynamoDB TTL.
type UserVerification struct {
	UserID    string `json:"user_id" dynamodbav:"user_id"`
	Type      string `json:"type" dynamodbav:"type"` // "otp" | "email"
	Code      string `json:"code" dynamodbav:"code"`
	ExpiresAt int64  `json:"expires_at" dynamodbav:"expires_at"` // TTL (Unix seconds)
}
