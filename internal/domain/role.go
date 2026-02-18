package domain

type Role struct {
	RoleID     string   `json:"id" dynamodbav:"role_id"`
	Name       string   `json:"name" dynamodbav:"name"`
	Enable     bool     `json:"enable" dynamodbav:"enable"`
	RoleAccess []string `json:"role_access,omitempty" dynamodbav:"role_access"`
}

type RoleInput struct {
	Name   string `json:"name" validate:"required"`
	Enable *bool  `json:"enable"`
}
