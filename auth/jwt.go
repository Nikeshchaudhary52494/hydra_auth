package main

import (
	"fmt"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid" // You need to install this: go get github.com/google/uuid
)

var SecretKey = os.Getenv("JWT_SECRET")

func init() {
	if SecretKey == "" {
		panic("JWT_SECRET environment variable is not set!")
	}
}

// Claims defines the structure for the Access Token (AT) payload
type Claims struct {
	UserID    int    `json:"user_id"`
	SessionID string `json:"session_id"` // NEW: Unique ID for this session/device
	jwt.RegisteredClaims
}

// TokensResponse holds both the Access and Refresh Tokens
type TokensResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

// generateTokens creates both the Access Token (AT) and Refresh Token (RT)
func generateTokens(userID int) (TokensResponse, error) {
	// 1. Generate unique Session ID
	sessionID := uuid.New().String()

	// 2. Access Token (Short-lived, contains session_id)
	accessToken, err := generateJWT(userID, sessionID)
	if err != nil {
		return TokensResponse{}, err
	}

	// 3. Refresh Token (Long-lived, random string)
	refreshToken := uuid.New().String() // RT is a simple unique string
	rtExpiration := 7 * 24 * time.Hour

	// 4. Store Refresh Token in Redis (Stateful session management starts here)
	// Key: user:{UserID}:sessions:{SessionID}
	// Value: RefreshToken (or metadata in Phase 3)
	redisKey := fmt.Sprintf("session:%s", sessionID)

	// We store the refresh token itself in Redis for verification
	if err := RedisClient.Set(RedisClient.Context(), redisKey, refreshToken, rtExpiration).Err(); err != nil {
		return TokensResponse{}, fmt.Errorf("failed to save refresh token to redis: %w", err)
	}

	return TokensResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, nil
}

// generateJWT creates a signed JWT for the given user ID and session ID
func generateJWT(userID int, sessionID string) (string, error) {
	expirationTime := time.Now().Add(15 * time.Minute) // 15-minute validity for AT

	claims := &Claims{
		UserID:    userID,
		SessionID: sessionID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Subject:   "access_token",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(SecretKey))
}
