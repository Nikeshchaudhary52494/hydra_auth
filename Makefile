.PHONY: all run proto migrate_up migrate_down create_migration clean setup

# --- Configuration ---
SERVICE_NAME = hydra-auth
AUTH_DIR = auth
AUTH_PKG = ./auth
PROTO_DIR = proto
PROTO_FILES = $(wildcard $(PROTO_DIR)/*.proto)
MIGRATION_DIR = migrations
# Migration tool
MIGRATE = migrate
OUT_DIR = pb

PROTOC_GEN_GO := $(shell which protoc-gen-go)
PROTOC_GEN_GO_GRPC := $(shell which protoc-gen-go-grpc)

# Include environment variables from .env file
include .env
export DB_URL
export JWT_SECRET
export AUTH_SERVICE_PORT
export REDIS_ADDR
export GRPC_AUTH_PORT

# --- Core Commands ---

all: proto run


run: ## Run the Auth service
	@echo "Starting $(SERVICE_NAME) on port $(AUTH_SERVICE_PORT)..."
	@cd $(AUTH_DIR) && go run .

# --- Protobuf & gRPC ---

proto:
	@if [ -z "$(PROTOC_GEN_GO)" ] || [ -z "$(PROTOC_GEN_GO_GRPC)" ]; then \
		echo "Installing protoc-gen-go and protoc-gen-go-grpc..."; \
		go install google.golang.org/protobuf/cmd/protoc-gen-go@latest; \
		go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest; \
	fi
	@echo "Generating Go code from .proto files..."
	 @protoc --proto_path=$(PROTO_DIR) \
        --go_out=$(OUT_DIR) \
        --go-grpc_out=$(OUT_DIR) \
        $(PROTO_FILES)


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