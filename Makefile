.PHONY: build run dry-run test clean install help tidy check lint fmt dev docker-build docker-run docker-push docker-compose-up docker-compose-down k8s-deploy k8s-delete helm-install helm-upgrade helm-uninstall release test-coverage test-race init topic

# Variables
APP_NAME=autoblog-ai
VERSION?=$(shell cat VERSION 2>/dev/null || echo "dev")
DOCKER_IMAGE=$(APP_NAME):$(VERSION)
DOCKER_REGISTRY?=ghcr.io/yourusername

# Colors for output
CYAN=\033[0;36m
GREEN=\033[0;32m
YELLOW=\033[1;33m
RED=\033[0;31m
NC=\033[0m

## Development Commands

# Full pre-commit check
check: tidy fmt lint test
	@echo "$(GREEN)✅ All checks passed!$(NC)"

# Tidy dependencies
tidy:
	@echo "$(CYAN)📦 Tidying Go modules...$(NC)"
	@go mod tidy
	@go mod verify
	@echo "$(GREEN)✅ Modules tidied$(NC)"

# Format code
fmt:
	@echo "$(CYAN)🎨 Formatting code...$(NC)"
	@go fmt ./...
	@echo "$(GREEN)✅ Code formatted$(NC)"

# Lint code
lint:
	@echo "$(CYAN)🔍 Linting code...$(NC)"
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run --timeout=5m; \
		echo "$(GREEN)✅ Linting passed$(NC)"; \
	else \
		echo "$(YELLOW)⚠️  golangci-lint not installed. Install: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest$(NC)"; \
	fi

# Run tests
test:
	@echo "$(CYAN)🧪 Running tests...$(NC)"
	@go test -v ./internal/...
	@echo "$(GREEN)✅ Tests passed$(NC)"

# Run tests with coverage
test-coverage:
	@echo "$(CYAN)📊 Running tests with coverage...$(NC)"
	@go test -v -coverprofile=coverage.out ./internal/...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "$(GREEN)✅ Coverage report: coverage.html$(NC)"

# Run tests with race detector
test-race:
	@echo "$(CYAN)🏁 Running tests with race detector...$(NC)"
	@go test -v -race ./internal/...
	@echo "$(GREEN)✅ Race tests passed$(NC)"

# Install dependencies
install:
	@echo "$(CYAN)📥 Installing dependencies...$(NC)"
	@go mod download
	@go mod tidy
	@echo "$(GREEN)✅ Dependencies installed$(NC)"

# Build the application
build: tidy
	@echo "$(CYAN)🔨 Building $(APP_NAME)...$(NC)"
	@go build -ldflags "-X main.Version=$(VERSION)" -o $(APP_NAME) .
	@echo "$(GREEN)✅ Build complete: ./$(APP_NAME)$(NC)"

# Run the application locally (publishes to Medium)
run: build
	@echo "$(CYAN)🚀 Running $(APP_NAME)...$(NC)"
	@./$(APP_NAME)

# Run in dry-run mode (preview without publishing)
dry-run: build
	@echo "$(CYAN)👁️  Running in dry-run mode...$(NC)"
	@./$(APP_NAME) --dry-run

# Run with specific topic
topic: build
	@if [ -z "$(TOPIC)" ]; then \
		echo "$(RED)❌ Error: TOPIC is required. Usage: make topic TOPIC='Your Topic'$(NC)"; \
		exit 1; \
	fi
	@echo "$(CYAN)📝 Generating article: $(TOPIC)$(NC)"
	@./$(APP_NAME) --topic "$(TOPIC)" --dry-run

# Local development with auto-reload
dev:
	@if command -v air >/dev/null 2>&1; then \
		echo "$(CYAN)🔄 Running with air (hot reload)...$(NC)"; \
		air; \
	elif command -v entr >/dev/null 2>&1; then \
		echo "$(CYAN)🔄 Running with entr (hot reload)...$(NC)"; \
		find . -name '*.go' | entr -r go run main.go --dry-run; \
	else \
		echo "$(YELLOW)⚠️  Install 'air' for hot reload: go install github.com/air-verse/air@latest$(NC)"; \
		make run; \
	fi

# Initialize project
init:
	@if [ ! -f .env ]; then \
		cp .env.example .env; \
		echo "$(GREEN)✅ Created .env file. Edit with your API keys.$(NC)"; \
	else \
		echo "$(YELLOW)ℹ️  .env file exists$(NC)"; \
	fi
	@make install

# Clean build artifacts
clean:
	@echo "$(CYAN)🧹 Cleaning...$(NC)"
	@rm -f $(APP_NAME)
	@rm -rf generated/
	@rm -f coverage.out coverage.html
	@echo "$(GREEN)✅ Cleaned$(NC)"

## Docker Commands

# Build Docker image
docker-build:
	@echo "$(CYAN)🐳 Building Docker image...$(NC)"
	@docker build -t $(DOCKER_IMAGE) -t $(APP_NAME):latest .
	@echo "$(GREEN)✅ Image built: $(DOCKER_IMAGE)$(NC)"

# Run in Docker
docker-run: docker-build
	@echo "$(CYAN)🐳 Running in Docker...$(NC)"
	@if [ ! -f .env ]; then \
		echo "$(RED)❌ .env file not found. Run 'make init'$(NC)"; \
		exit 1; \
	fi
	@docker run --rm \
		--env-file .env \
		-v $(PWD)/config.yaml:/app/config.yaml:ro \
		-v $(PWD)/topics.csv:/app/topics.csv:ro \
		-v $(PWD)/templates:/app/templates:ro \
		-v $(PWD)/generated:/app/generated \
		$(DOCKER_IMAGE) --dry-run

# Push Docker image
docker-push: docker-build
	@echo "$(CYAN)🐳 Pushing to registry...$(NC)"
	@docker tag $(DOCKER_IMAGE) $(DOCKER_REGISTRY)/$(DOCKER_IMAGE)
	@docker push $(DOCKER_REGISTRY)/$(DOCKER_IMAGE)
	@echo "$(GREEN)✅ Pushed: $(DOCKER_REGISTRY)/$(DOCKER_IMAGE)$(NC)"

# Start with docker-compose
docker-compose-up:
	@echo "$(CYAN)🐳 Starting services...$(NC)"
	@docker-compose up -d
	@echo "$(GREEN)✅ Services started$(NC)"

# Stop docker-compose
docker-compose-down:
	@echo "$(CYAN)🐳 Stopping services...$(NC)"
	@docker-compose down
	@echo "$(GREEN)✅ Services stopped$(NC)"

## Kubernetes Commands

# Deploy to Kubernetes
k8s-deploy:
	@echo "$(CYAN)☸️  Deploying to Kubernetes...$(NC)"
	@kubectl apply -f k8s/
	@echo "$(GREEN)✅ Deployed$(NC)"

# Delete from Kubernetes
k8s-delete:
	@echo "$(CYAN)☸️  Deleting from Kubernetes...$(NC)"
	@kubectl delete -f k8s/
	@echo "$(GREEN)✅ Deleted$(NC)"

# Kubernetes status
k8s-status:
	@echo "$(CYAN)☸️  Checking status...$(NC)"
	@kubectl get pods,svc,cronjobs -l app=$(APP_NAME)

# Kubernetes logs
k8s-logs:
	@kubectl logs -l app=$(APP_NAME) --tail=100 -f

## Helm Commands

# Install with Helm
helm-install:
	@echo "$(CYAN)⎈ Installing with Helm...$(NC)"
	@helm install $(APP_NAME) ./helm/autoblog-ai \
		--set image.tag=$(VERSION) \
		--create-namespace \
		--namespace autoblog-ai
	@echo "$(GREEN)✅ Installed$(NC)"

# Upgrade with Helm
helm-upgrade:
	@echo "$(CYAN)⎈ Upgrading with Helm...$(NC)"
	@helm upgrade $(APP_NAME) ./helm/autoblog-ai \
		--set image.tag=$(VERSION) \
		--namespace autoblog-ai
	@echo "$(GREEN)✅ Upgraded$(NC)"

# Uninstall from Helm
helm-uninstall:
	@echo "$(CYAN)⎈ Uninstalling from Helm...$(NC)"
	@helm uninstall $(APP_NAME) --namespace autoblog-ai
	@echo "$(GREEN)✅ Uninstalled$(NC)"

# Helm status
helm-status:
	@helm status $(APP_NAME) --namespace autoblog-ai

## Release Commands

# Create release tag (manual)
release:
	@if [ -z "$(VERSION)" ] || [ "$(VERSION)" = "dev" ]; then \
		echo "$(RED)❌ Error: VERSION required. Usage: make release VERSION=1.0.0$(NC)"; \
		exit 1; \
	fi
	@echo "$(CYAN)📦 Creating release v$(VERSION)...$(NC)"
	@echo "$(VERSION)" > VERSION
	@git add VERSION
	@git commit -m "chore: bump version to $(VERSION)"
	@git tag -a v$(VERSION) -m "Release v$(VERSION)"
	@git push origin main
	@git push origin v$(VERSION)
	@echo "$(GREEN)✅ Released v$(VERSION)$(NC)"

## Help

# Show help
help:
	@echo ""
	@echo "$(CYAN)╔══════════════════════════════════════════════════════╗$(NC)"
	@echo "$(CYAN)║          AutoBlog AI - Makefile Commands            ║$(NC)"
	@echo "$(CYAN)╚══════════════════════════════════════════════════════╝$(NC)"
	@echo ""
	@echo "$(GREEN)📋 Development:$(NC)"
	@echo "  make check           - Run all checks (tidy + fmt + lint + test)"
	@echo "  make tidy            - Tidy and verify Go modules"
	@echo "  make fmt             - Format code"
	@echo "  make lint            - Lint with golangci-lint"
	@echo "  make test            - Run tests"
	@echo "  make test-coverage   - Generate coverage report"
	@echo "  make test-race       - Test with race detector"
	@echo "  make install         - Install dependencies"
	@echo "  make init            - Initialize project (.env)"
	@echo "  make dev             - Run with hot reload"
	@echo ""
	@echo "$(GREEN)🔨 Build & Run:$(NC)"
	@echo "  make build           - Build binary"
	@echo "  make run             - Run locally (publishes!)"
	@echo "  make dry-run         - Run in preview mode"
	@echo "  make topic TOPIC='X' - Generate specific topic"
	@echo "  make clean           - Clean artifacts"
	@echo ""
	@echo "$(GREEN)🐳 Docker:$(NC)"
	@echo "  make docker-build    - Build image"
	@echo "  make docker-run      - Run in container"
	@echo "  make docker-push     - Push to registry"
	@echo "  make docker-compose-up   - Start services"
	@echo "  make docker-compose-down - Stop services"
	@echo ""
	@echo "$(GREEN)☸️  Kubernetes:$(NC)"
	@echo "  make k8s-deploy      - Deploy to cluster"
	@echo "  make k8s-delete      - Delete from cluster"
	@echo "  make k8s-status      - Check status"
	@echo "  make k8s-logs        - View logs"
	@echo ""
	@echo "$(GREEN)⎈ Helm:$(NC)"
	@echo "  make helm-install    - Install chart"
	@echo "  make helm-upgrade    - Upgrade release"
	@echo "  make helm-uninstall  - Uninstall"
	@echo "  make helm-status     - Check status"
	@echo ""
	@echo "$(GREEN)📦 Release:$(NC)"
	@echo "  make release VERSION=1.0.0 - Create release"
	@echo ""
	@echo "$(YELLOW)Examples:$(NC)"
	@echo "  make check                      # Pre-commit checks"
	@echo "  make dry-run                    # Test locally"
	@echo "  make topic TOPIC='Go Patterns'  # Specific topic"
	@echo "  make docker-run                 # Test in Docker"
	@echo "  make k8s-deploy                 # Deploy to K8s"
	@echo ""
