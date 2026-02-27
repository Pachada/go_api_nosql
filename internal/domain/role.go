package domain

// Role name constants — used for RBAC checks across the application.
const (
	RoleAdmin = "Admin"
	RoleUser  = "User"
)

// AuthProvider constants identify how a user account was created.
const (
	AuthProviderLocal  = "local"
	AuthProviderGoogle = "google"
)
