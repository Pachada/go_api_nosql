package domain

type Status struct {
	StatusID    string `json:"id" dynamodbav:"status_id"`
	Description string `json:"description" dynamodbav:"description"`
}

type StatusInput struct {
	Description string `json:"description" validate:"required"`
}
