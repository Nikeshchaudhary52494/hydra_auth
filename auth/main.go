package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"sync"
	"time"

	proto "hydraauth/auth/pb/authpb" // Import the generated protobuf package

	"github.com/go-redis/redis/v8" // Using v8 context methods
	"github.com/golang-jwt/jwt/v5" // Using the V5 JWT package
	_ "github.com/lib/pq"
	"google.golang.org/grpc"
)

// Global database connection pool
var DB *sql.DB
var RedisClient *redis.Client

// NOTE: SecretKey and Claims struct are expected to be defined in auth/jwt.go
// and accessible here (either by being in the same package 'main' or via import).
// Assuming they are defined in jwt.go and belong to the package 'main'.

func main() {
	// --- 1. Database Connection ---
	connStr := os.Getenv("DB_URL")
	if connStr == "" {
		log.Fatal("DB_URL environment variable is not set.")
	}

	var err error
	DB, err = sql.Open("postgres", connStr)
	if err != nil {
		log.Fatal("Error opening database connection:", err)
	}
	defer DB.Close()

	if err = DB.Ping(); err != nil {
		log.Fatal("Error connecting to database:", err)
	}
	log.Println("Successfully connected to PostgreSQL!")

	// --- 2. Redis Connection ---
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		log.Fatal("REDIS_ADDR environment variable is not set.")
	}
	RedisClient = redis.NewClient(&redis.Options{
		Addr: redisAddr,
	})
	if err := RedisClient.Ping(RedisClient.Context()).Err(); err != nil {
		log.Fatalf("Could not connect to Redis: %v", err)
	}
	log.Println("Successfully connected to Redis!")

	// --- 3. Run Servers Concurrently ---
	var wg sync.WaitGroup

	// Start HTTP Server
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := runHTTPServer(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server failed: %v", err)
		}
	}()

	// Start gRPC Server
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := runGRPCServer(); err != nil {
			log.Fatalf("gRPC server failed: %v", err)
		}
	}()

	wg.Wait()
}

// runHTTPServer starts the public-facing API server (Login/Register)
func runHTTPServer() error {
	// NOTE: RegisterHandler, LoginHandler, RefreshHandler must be defined in handlers.go
	router := http.NewServeMux()
	router.HandleFunc("/auth/register", RegisterHandler)
	router.HandleFunc("/auth/login", LoginHandler)
	router.HandleFunc("/auth/refresh", RefreshHandler)

	port := os.Getenv("AUTH_SERVICE_PORT")
	if port == "" {
		port = "8080"
	}
	listenAddr := ":" + port

	server := &http.Server{
		Addr:         listenAddr,
		Handler:      router,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  15 * time.Second,
	}

	fmt.Printf("HTTP Auth Service listening on %s...\n", listenAddr)
	return server.ListenAndServe()
}

// runGRPCServer starts the internal gRPC server (ValidateToken)
func runGRPCServer() error {
	grpcPort := os.Getenv("GRPC_AUTH_PORT")
	if grpcPort == "" {
		grpcPort = "50051"
	}

	lis, err := net.Listen("tcp", ":"+grpcPort)
	if err != nil {
		return fmt.Errorf("failed to listen on port %s: %w", grpcPort, err)
	}

	grpcServer := grpc.NewServer()

	// Register the AuthValidation server implementation
	proto.RegisterAuthValidationServer(grpcServer, &AuthValidationServer{})

	fmt.Printf("gRPC Auth Service listening on :%s...\n", grpcPort)
	return grpcServer.Serve(lis)
}

// AuthValidationServer implements the gRPC interface defined in auth.proto
type AuthValidationServer struct {
	// FIX: Embed the generated unimplemented server struct
	proto.UnimplementedAuthValidationServer
}

// ValidateToken implements the rpc from the proto file
func (s *AuthValidationServer) ValidateToken(ctx context.Context, req *proto.ValidateTokenRequest) (*proto.ValidateTokenResponse, error) {
	// 1. Stateless JWT Validation (Signature and Expiry)
	// NOTE: Claims and SecretKey must be accessible (from jwt.go)
	claims := &Claims{}

	token, err := jwt.ParseWithClaims(req.Token, claims, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(SecretKey), nil
	})

	if err != nil || !token.Valid {
		return &proto.ValidateTokenResponse{
			IsValid: false,
			Error:   "Token is invalid or expired: " + err.Error(),
		}, nil
	}

	// 2. Stateful Session Check (Required for device limit/revocation)
	redisKey := fmt.Sprintf("session:%s", claims.SessionID)

	// Check for existence of the session in Redis
	_, err = RedisClient.Get(ctx, redisKey).Result()

	if err == redis.Nil {
		// Session revoked or timed out
		return &proto.ValidateTokenResponse{
			IsValid: false,
			Error:   "Session revoked or not active (SessionID not found in Redis).",
		}, nil
	} else if err != nil {
		log.Printf("Redis check error: %v", err)
		return &proto.ValidateTokenResponse{
			IsValid: false,
			Error:   "Internal server error during session check.",
		}, nil
	}

	// 3. Successful Validation
	return &proto.ValidateTokenResponse{
		IsValid: true,
		UserId:  int32(claims.UserID),
		Error:   "",
	}, nil
}
