#!/bin/bash

# Verification script for AutoBlog AI infrastructure
# This script checks that all components are properly configured

set -e

# Change to script directory
cd "$(dirname "$0")"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

ERRORS=0
WARNINGS=0

echo -e "${CYAN}╔══════════════════════════════════════════════════════╗${NC}"
echo -e "${CYAN}║       AutoBlog AI - Infrastructure Verification      ║${NC}"
echo -e "${CYAN}╚══════════════════════════════════════════════════════╝${NC}"
echo ""

# Function to print success
success() {
    echo -e "${GREEN}✓${NC} $1"
}

# Function to print error
error() {
    echo -e "${RED}✗${NC} $1"
    ERRORS=$((ERRORS + 1))
}

# Function to print warning
warning() {
    echo -e "${YELLOW}⚠${NC} $1"
    WARNINGS=$((WARNINGS + 1))
}

# Function to print info
info() {
    echo -e "${CYAN}ℹ${NC} $1"
}

echo -e "${CYAN}[1/8] Checking Go environment...${NC}"
if command -v go >/dev/null 2>&1; then
    GO_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
    if [ "$(echo "$GO_VERSION" | cut -d. -f1)" -ge 1 ] && [ "$(echo "$GO_VERSION" | cut -d. -f2)" -ge 25 ]; then
        success "Go $GO_VERSION installed"
    else
        error "Go version $GO_VERSION is too old (need 1.25+)"
    fi
else
    error "Go not found"
fi

echo ""
echo -e "${CYAN}[2/8] Checking project structure...${NC}"

REQUIRED_FILES=(
    "go.mod"
    "go.sum"
    "main.go"
    "config.yaml"
    "topics.csv"
    "Makefile"
    "Dockerfile"
    ".dockerignore"
    "docker-compose.yml"
    ".golangci.yml"
    ".env.example"
    "templates/system-prompt.md"
    "templates/article-prompt.md"
)

for file in "${REQUIRED_FILES[@]}"; do
    if [ -f "$file" ]; then
        success "$file exists"
    else
        error "$file missing"
    fi
done

REQUIRED_DIRS=(
    "internal/article"
    "internal/config"
    "internal/medium"
    "internal/storage"
    ".github/workflows"
    "k8s"
    "helm/autoblog-ai/templates"
)

for dir in "${REQUIRED_DIRS[@]}"; do
    if [ -d "$dir" ]; then
        success "$dir/ exists"
    else
        error "$dir/ missing"
    fi
done

echo ""
echo -e "${CYAN}[3/8] Checking Go modules...${NC}"
if go mod verify >/dev/null 2>&1; then
    success "Go modules verified"
else
    error "Go modules verification failed"
fi

echo ""
echo -e "${CYAN}[4/8] Running tests...${NC}"
if go test -v ./internal/... >/dev/null 2>&1; then
    TEST_COUNT=$(go test ./internal/... 2>&1 | grep -c "PASS:")
    success "All tests passed ($TEST_COUNT tests)"
else
    error "Tests failed"
fi

echo ""
echo -e "${CYAN}[5/8] Checking build...${NC}"
if go build -o autoblog-ai-verify . >/dev/null 2>&1; then
    success "Build successful"
    rm -f autoblog-ai-verify
else
    error "Build failed"
fi

echo ""
echo -e "${CYAN}[6/8] Checking Docker setup...${NC}"
if [ -f "Dockerfile" ]; then
    success "Dockerfile exists"

    # Validate Dockerfile structure
    if grep -q "FROM golang:1.25" Dockerfile; then
        success "Dockerfile uses Go 1.25"
    else
        warning "Dockerfile may not use Go 1.25"
    fi

    if grep -q "FROM alpine" Dockerfile; then
        success "Dockerfile uses multi-stage build"
    else
        warning "Dockerfile may not use multi-stage build"
    fi
else
    error "Dockerfile missing"
fi

if [ -f ".dockerignore" ]; then
    success ".dockerignore exists"
else
    warning ".dockerignore missing (build will be slower)"
fi

if [ -f "docker-compose.yml" ]; then
    success "docker-compose.yml exists"
else
    warning "docker-compose.yml missing"
fi

echo ""
echo -e "${CYAN}[7/8] Checking Kubernetes manifests...${NC}"

K8S_FILES=(
    "k8s/namespace.yaml"
    "k8s/secret.yaml"
    "k8s/configmap.yaml"
    "k8s/cronjob.yaml"
    "k8s/deployment.yaml"
)

for file in "${K8S_FILES[@]}"; do
    if [ -f "$file" ]; then
        success "$file exists"
    else
        error "$file missing"
    fi
done

echo ""
echo -e "${CYAN}[8/8] Checking Helm chart...${NC}"

HELM_FILES=(
    "helm/autoblog-ai/Chart.yaml"
    "helm/autoblog-ai/values.yaml"
    "helm/autoblog-ai/templates/_helpers.tpl"
    "helm/autoblog-ai/templates/NOTES.txt"
    "helm/autoblog-ai/templates/secret.yaml"
    "helm/autoblog-ai/templates/configmap.yaml"
    "helm/autoblog-ai/templates/cronjob.yaml"
    "helm/autoblog-ai/templates/deployment.yaml"
)

for file in "${HELM_FILES[@]}"; do
    if [ -f "$file" ]; then
        success "$file exists"
    else
        error "$file missing"
    fi
done

# Check if helm is installed for validation
if command -v helm >/dev/null 2>&1; then
    info "Running helm lint..."
    if helm lint helm/autoblog-ai >/dev/null 2>&1; then
        success "Helm chart validation passed"
    else
        warning "Helm chart validation has warnings"
    fi
else
    warning "Helm not installed (skipping chart validation)"
fi

echo ""
echo -e "${CYAN}═══════════════════════════════════════════════════════${NC}"
echo -e "${CYAN}                    Summary                             ${NC}"
echo -e "${CYAN}═══════════════════════════════════════════════════════${NC}"

if [ $ERRORS -eq 0 ] && [ $WARNINGS -eq 0 ]; then
    echo -e "${GREEN}✓ All checks passed!${NC}"
    echo ""
    echo "You're ready to deploy. Next steps:"
    echo "1. Set up .env with your API keys"
    echo "2. Test locally: make dry-run"
    echo "3. Deploy with Docker: make docker-run"
    echo "4. Or deploy to K8s: make k8s-deploy"
    echo "5. Or use Helm: make helm-install"
    exit 0
elif [ $ERRORS -eq 0 ]; then
    echo -e "${YELLOW}⚠ $WARNINGS warning(s) found${NC}"
    echo ""
    echo "Warnings don't block deployment but should be reviewed."
    exit 0
else
    echo -e "${RED}✗ $ERRORS error(s) found${NC}"
    if [ $WARNINGS -gt 0 ]; then
        echo -e "${YELLOW}⚠ $WARNINGS warning(s) found${NC}"
    fi
    echo ""
    echo "Please fix the errors before deploying."
    exit 1
fi
