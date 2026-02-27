package google

import (
	"context"
	"fmt"

	"github.com/go-api-nosql/internal/domain"
	"google.golang.org/api/idtoken"
)

// Payload holds the verified claims extracted from a Google ID token.
type Payload struct {
	Sub           string
	Email         string
	EmailVerified bool
	FirstName     string
	LastName      string
}

// Verifier verifies Google ID tokens against a specific client ID.
type Verifier struct {
	clientID string
}

func NewVerifier(clientID string) *Verifier {
	return &Verifier{clientID: clientID}
}

// Verify validates the Google ID token and returns the extracted payload.
// Returns a domain.ErrUnauthorized-wrapped error if the token is invalid.
func (v *Verifier) Verify(ctx context.Context, token string) (*Payload, error) {
	p, err := idtoken.Validate(ctx, token, v.clientID)
	if err != nil {
		return nil, fmt.Errorf("invalid google token: %w", domain.ErrUnauthorized)
	}
	email, _ := p.Claims["email"].(string)
	emailVerified, _ := p.Claims["email_verified"].(bool)
	firstName, _ := p.Claims["given_name"].(string)
	lastName, _ := p.Claims["family_name"].(string)
	return &Payload{
		Sub:           p.Subject,
		Email:         email,
		EmailVerified: emailVerified,
		FirstName:     firstName,
		LastName:      lastName,
	}, nil
}
