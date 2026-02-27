package domain

import "time"

type User struct {
	UserID         string     `json:"id" dynamodbav:"user_id"`
	Username       string     `json:"username" dynamodbav:"username"`
	Email          string     `json:"email" dynamodbav:"email"`
	Phone          *string    `json:"phone" dynamodbav:"phone"`
	PasswordHash   string     `json:"-" dynamodbav:"password_hash"`
	Role           string     `json:"role" dynamodbav:"role"`
	FirstName      string     `json:"first_name" dynamodbav:"first_name"`
	LastName       string     `json:"last_name" dynamodbav:"last_name"`
	Birthday       time.Time  `json:"birthday" dynamodbav:"birthday"`
	Verified       bool       `json:"verified" dynamodbav:"verified"`
	EmailConfirmed bool       `json:"email_confirmed" dynamodbav:"email_confirmed"`
	PhoneConfirmed bool       `json:"phone_confirmed" dynamodbav:"phone_confirmed"`
	AuthProvider   string     `json:"auth_provider,omitempty" dynamodbav:"auth_provider"` // "local" | "google"
	GoogleSub      string     `json:"-"                       dynamodbav:"google_sub"`
	Enable         int        `json:"enable" dynamodbav:"enable"`
	DeletedAt      *time.Time `json:"deleted_at,omitempty" dynamodbav:"deleted_at"`
	CreatedAt      time.Time  `json:"created" dynamodbav:"created_at"`
	UpdatedAt      time.Time  `json:"updated" dynamodbav:"updated_at"`
}

type CreateUserRequest struct {
	Username   string  `json:"username" validate:"required"`
	Password   string  `json:"password" validate:"required,min=8,max=72"`
	Email      string  `json:"email" validate:"required,email"`
	Phone      *string `json:"phone"`
	FirstName  string  `json:"first_name" validate:"required"`
	LastName   string  `json:"last_name" validate:"required"`
	Birthday   string  `json:"birthday"` // expected format: YYYY-MM-DD
	DeviceUUID *string `json:"device_uuid"`
}

type UpdateUserRequest struct {
	Username  *string `json:"username"`
	Email     *string `json:"email" validate:"omitempty,email"`
	Phone     *string `json:"phone"`
	FirstName *string `json:"first_name"`
	LastName  *string `json:"last_name"`
	Birthday  *string `json:"birthday"` // expected format: YYYY-MM-DD
	Role      *string `json:"role"`
	Enable    *int    `json:"enable"` // 1 = enabled, 0 = disabled
}
