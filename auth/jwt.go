package main

import (
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// SecretKey is used for signing the JWTs.
// In a real application, this MUST be read from an environment variable or secure vault.
var SecretKey = os.Getenv("JWT_SECRET")

func init() {
	if SecretKey == "" {
		// This is a safety check. The Makefile ensures it's set locally.
		panic("JWT_SECRET environment variable is not set!")
	}
}

// Claims defines the structure for the JWT payload
type Claims struct {
	UserID int `json:"user_id"`
	// Phase 2 addition: SessionID string `json:"session_id"`
	jwt.RegisteredClaims
}

// generateJWT creates a signed JWT for the given user ID
func generateJWT(userID int) (string, error) {
	expirationTime := time.Now().Add(15 * time.Minute) // Access Token lifetime

	claims := &Claims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Subject:   "access_token",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(SecretKey))

	return tokenString, err
}

// ---------------------------------------------------------------------
// Phase 1: Only need generation. Validation logic moves to the API Gateway.
// ---------------------------------------------------------------------
