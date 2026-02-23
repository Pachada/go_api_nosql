package jwtinfra

import (
	"crypto/rsa"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/go-api-nosql/internal/config"
	"github.com/golang-jwt/jwt/v5"
)

// Claims holds the JWT payload fields.
type Claims struct {
	UserID    string `json:"user_id"`
	DeviceID  string `json:"device_id"`
	Role      string `json:"role"`
	SessionID string `json:"session_id"`
	jwt.RegisteredClaims
}

// Provider signs and verifies RS256 JWTs.
type Provider struct {
	privateKey *rsa.PrivateKey
	publicKey  *rsa.PublicKey
	expiry     time.Duration
}

func NewProvider(cfg *config.Config) (*Provider, error) {
	privBytes, err := os.ReadFile(cfg.JWTPrivateKeyPath)
	if err != nil {
		return nil, fmt.Errorf("read private key: %w", err)
	}
	privKey, err := jwt.ParseRSAPrivateKeyFromPEM(privBytes)
	if err != nil {
		return nil, fmt.Errorf("parse private key: %w", err)
	}

	pubBytes, err := os.ReadFile(cfg.JWTPublicKeyPath)
	if err != nil {
		return nil, fmt.Errorf("read public key: %w", err)
	}
	pubKey, err := jwt.ParseRSAPublicKeyFromPEM(pubBytes)
	if err != nil {
		return nil, fmt.Errorf("parse public key: %w", err)
	}

	return &Provider{privateKey: privKey, publicKey: pubKey, expiry: cfg.JWTExpiry}, nil
}

func (p *Provider) Sign(userID, deviceID, role, sessionID string) (string, error) {
	claims := Claims{
		UserID:    userID,
		DeviceID:  deviceID,
		Role:      role,
		SessionID: sessionID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(p.expiry)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	return token.SignedString(p.privateKey)
}

func (p *Provider) Verify(tokenStr string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return p.publicKey, nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token claims")
	}
	return claims, nil
}
