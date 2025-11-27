.PHONY: all run proto migrate_up migrate_down create_migration clean setup

# --- Configuration ---
SERVICE_NAME = hydra-auth
AUTH_DIR = auth
AUTH_PKG = ./auth
PROTO_DIR = proto
MIGRATION_DIR = migrations
# Migration tool
MIGRATE = migrate

# Include environment variables from .env file
include .env
export DB_URL
export JWT_SECRET
export AUTH_SERVICE_PORT

# --- Core Commands ---

all: proto run


run: ## Run the Auth service
	@echo "Starting $(SERVICE_NAME) on port $(AUTH_SERVICE_PORT)..."
	@cd $(AUTH_DIR) && go run .

# --- Protobuf & gRPC ---

proto: ## Generate Go code from Protobuf files
	@echo "Generating Protobuf code..."
	protoc --go_out=. --go-grpc_out=. $(PROTO_DIR)/auth.proto

# --- Database Migrations ---

migrate_up: ## Apply all pending database migrations
	@echo "Applying migrations..."
	@$(MIGRATE) -path $(MIGRATION_DIR) -database $(DB_URL) up

migrate_down: ## Rollback the last applied migration
	@echo "Rolling back last migration..."
	@$(MIGRATE) -path $(MIGRATION_DIR) -database $(DB_URL) down

create_migration: ## Create new migration files (Usage: make create_migration NAME=add_user_role)
ifndef NAME
	$(error NAME is required. Usage: make create_migration NAME=my_migration)
endif
	@echo "Creating migration files for: $(NAME)"
	@$(MIGRATE) create -ext sql -dir $(MIGRATION_DIR) -seq $(NAME)

# --- Utility ---

clean: ## Clean up generated files
	@echo "Cleaning up generated files..."
	rm -f $(PROTO_DIR)/*.pb.go $(PROTO_DIR)/*_grpc.pb.go

setup: ## Install necessary Go tools
	@echo "Installing tools..."
	go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
	go get google.golang.org/grpc
	go get google.golang.org/protobuf

help: ## Show this help message
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'