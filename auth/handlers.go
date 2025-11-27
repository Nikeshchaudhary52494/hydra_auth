package main

import (
	"context" // Needed for RedisClient.Context()
	"database/sql"
	"encoding/json"
	"fmt"
	"log" // Needed for logging errors
	"net/http"

	// Use the official v8 client import path
	"github.com/go-redis/redis/v8"

	// Use the official v5 JWT import path
	"github.com/golang-jwt/jwt/v5"

	"golang.org/x/crypto/bcrypt"
)

// NOTE: These variables are declared in auth/main.go but used here.
// They must be accessible (e.g., declared as 'var DB *sql.DB' in main.go).
// Assuming they are defined in main.go:
// var DB *sql.DB
// var RedisClient *redis.Client

// --- Handlers ---

// RegisterRequest defines the expected structure for registration
type RegisterRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// User defines the structure for a user record
type User struct {
	ID           int
	Email        string
	PasswordHash string
}

// RegisterHandler handles new user creation
func RegisterHandler(w http.ResponseWriter, r *http.Request) {
	// ... (Registration logic remains correct)
	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("Error hashing password: %v", err)
		http.Error(w, "Failed to hash password", http.StatusInternalServerError)
		return
	}

	var userID int
	err = DB.QueryRow("INSERT INTO users (email, password_hash) VALUES ($1, $2) RETURNING id",
		req.Email, string(hashedPassword)).Scan(&userID)

	if err != nil {
		log.Printf("Error registering user: %v", err)
		http.Error(w, "Registration failed, email might already exist", http.StatusConflict)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{"message": "User registered successfully", "user_id": userID})
}

// LoginHandler handles user authentication and JWT generation
func LoginHandler(w http.ResponseWriter, r *http.Request) {
	// ... (Login logic remains correct)
	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	var user User
	err := DB.QueryRow("SELECT id, email, password_hash FROM users WHERE email = $1", req.Email).
		Scan(&user.ID, &user.Email, &user.PasswordHash)

	if err == sql.ErrNoRows {
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	} else if err != nil {
		log.Printf("Database error during login: %v", err)
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	tokens, err := generateTokens(user.ID) // Assumes generateTokens is defined in jwt.go
	if err != nil {
		log.Printf("Error generating tokens: %v", err)
		http.Error(w, "Failed to generate tokens", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(tokens)
}

// RefreshRequest defines the expected structure for token renewal
type RefreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

// RefreshHandler handles the renewal of access tokens using a refresh token
func RefreshHandler(w http.ResponseWriter, r *http.Request) {
	var req RefreshRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	// Fix 1: Client must pass the expired AT in the Authorization header to get the SessionID
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" || len(authHeader) < 8 || authHeader[:7] != "Bearer " {
		http.Error(w, "Authorization header (Bearer <AT>) required for refresh", http.StatusUnauthorized)
		return
	}
	tokenString := authHeader[7:]

	// 1. Decode the expired Access Token to get the Session ID
	claims := &Claims{}
	// jwt.ParseWithClaims requires the secret key, but we're only reading claims here.
	// If the AT is expired, jwt.ParseWithClaims will fail.
	// We use jwt.NewParser().ParseUnverified to extract claims from potentially expired AT.
	_, _, err := jwt.NewParser().ParseUnverified(tokenString, claims)
	if err != nil {
		log.Printf("Failed to parse expired AT: %v", err)
		http.Error(w, "Invalid Access Token structure", http.StatusUnauthorized)
		return
	}

	if claims.SessionID == "" {
		http.Error(w, "Token missing session ID claim", http.StatusUnauthorized)
		return
	}

	// 2. Use the Session ID to find the Refresh Token in Redis
	// Key: session:{SessionID}
	redisKey := fmt.Sprintf("session:%s", claims.SessionID)

	// We need to use RedisClient.Context() here
	storedRT, err := RedisClient.Get(context.Background(), redisKey).Result()

	if err == redis.Nil {
		http.Error(w, "Session expired or revoked", http.StatusUnauthorized)
		return
	} else if err != nil {
		log.Printf("Server error checking session in Redis: %v", err)
		http.Error(w, "Server error checking session", http.StatusInternalServerError)
		return
	}

	// 3. Compare the stored RT with the submitted RT
	if storedRT != req.RefreshToken {
		// Revoke the session since a mismatch implies an attack or error
		RedisClient.Del(context.Background(), redisKey)
		http.Error(w, "Invalid refresh token", http.StatusUnauthorized)
		return
	}

	// 4. Invalidate old Refresh Token (One-time use)
	RedisClient.Del(context.Background(), redisKey)

	// 5. Generate new Access and Refresh Tokens
	newTokens, err := generateTokens(claims.UserID)
	if err != nil {
		log.Printf("Failed to generate new tokens: %v", err)
		http.Error(w, "Failed to generate new tokens", http.StatusInternalServerError)
		return
	}

	// 6. Respond
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(newTokens)
}
