package dynamo

// DynamoDB attribute names used in update expressions across all repos.
// Using constants prevents silent runtime bugs caused by key typos.
const (
	fieldEnable           = "enable"
	fieldDeletedAt        = "deleted_at"
	fieldReaded           = "readed"
	fieldRefreshToken     = "refresh_token"
	fieldRefreshExpiresAt = "refresh_expires_at"
)
