package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	_ "github.com/lib/pq"
)

// Global database connection pool
var DB *sql.DB

func main() {
	// --- 1. Database Connection ---
	connStr := os.Getenv("DB_URL") // Read from environment
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

	// --- 2. HTTP Routes ---
	router := http.NewServeMux()
	router.HandleFunc("/auth/register", RegisterHandler)
	router.HandleFunc("/auth/login", LoginHandler)

	// --- 3. Start Server ---
	port := os.Getenv("AUTH_SERVICE_PORT")
	if port == "" {
		port = ":8080" // Default if not set
	}
	server := &http.Server{
		Addr:         port,
		Handler:      router,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  15 * time.Second,
	}

	fmt.Printf("Auth Service listening on port %s...\n", port)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Could not listen on %s: %v\n", port, err)
	}
}
