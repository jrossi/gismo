#!/bin/bash
# Development Environment Setup Script
# Automates the setup of a consistent development environment

set -euo pipefail

# Configuration
readonly SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly PROJECT_ROOT="$(git rev-parse --show-toplevel 2>/dev/null || pwd)"
readonly LOG_FILE="${PROJECT_ROOT}/.setup-dev.log"

# Colors for output
readonly RED='\033[0;31m'
readonly GREEN='\033[0;32m'
readonly YELLOW='\033[1;33m'
readonly BLUE='\033[0;34m'
readonly NC='\033[0m' # No Color

# Logging functions
log() {
    echo -e "${BLUE}[$(date +'%Y-%m-%d %H:%M:%S')]${NC} $*" | tee -a "${LOG_FILE}"
}

success() {
    echo -e "${GREEN}✓${NC} $*" | tee -a "${LOG_FILE}"
}

warning() {
    echo -e "${YELLOW}⚠${NC} $*" | tee -a "${LOG_FILE}"
}

error() {
    echo -e "${RED}✗${NC} $*" | tee -a "${LOG_FILE}"
}

# Check if command exists
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

# Install tools based on platform
install_tool() {
    local tool="$1"
    local install_cmd="$2"
    
    if command_exists "$tool"; then
        success "$tool is already installed"
        return 0
    fi
    
    log "Installing $tool..."
    if eval "$install_cmd"; then
        success "$tool installed successfully"
    else
        error "Failed to install $tool"
        return 1
    fi
}

# Setup Git hooks
setup_git_hooks() {
    log "Setting up Git hooks..."
    
    local hooks_dir="${PROJECT_ROOT}/.git/hooks"
    if [[ ! -d "$hooks_dir" ]]; then
        warning "No .git/hooks directory found, skipping Git hooks setup"
        return 0
    fi
    
    # Pre-commit hook
    cat > "${hooks_dir}/pre-commit" << 'EOF'
#!/bin/bash
# Pre-commit hook for code quality checks

echo "Running pre-commit checks..."

# Check for syntax errors
if command -v gofmt >/dev/null 2>&1; then
    echo "Checking Go formatting..."
    if ! gofmt -l . | grep -q '^$'; then
        echo "❌ Go files need formatting. Run: gofmt -w ."
        exit 1
    fi
fi

# Check for common issues
echo "Checking for common issues..."
if grep -r "TODO\|FIXME\|XXX" --include="*.go" --include="*.js" --include="*.py" .; then
    echo "⚠️  Found TODO/FIXME comments in staged files"
fi

echo "✅ Pre-commit checks passed"
EOF

    chmod +x "${hooks_dir}/pre-commit"
    success "Git pre-commit hook installed"
}

# Setup development directories
setup_directories() {
    log "Setting up development directories..."
    
    local dirs=(
        "logs"
        "tmp"
        "data"
        "backups"
    )
    
    for dir in "${dirs[@]}"; do
        local path="${PROJECT_ROOT}/${dir}"
        if [[ ! -d "$path" ]]; then
            mkdir -p "$path"
            success "Created directory: $dir"
        else
            log "Directory already exists: $dir"
        fi
    done
}

# Setup environment files
setup_env_files() {
    log "Setting up environment files..."
    
    # Create .env.example if it doesn't exist
    local env_example="${PROJECT_ROOT}/.env.example"
    if [[ ! -f "$env_example" ]]; then
        cat > "$env_example" << 'EOF'
# Example environment variables
# Copy this file to .env and customize as needed

# Development settings
DEBUG=true
LOG_LEVEL=debug

# Database settings
DB_HOST=localhost
DB_PORT=5432
DB_NAME=myapp_dev
DB_USER=myapp_user
DB_PASSWORD=change_me

# API settings
API_PORT=8080
API_HOST=localhost

# Third-party services
# Add your API keys and service URLs here
EOF
        success "Created .env.example file"
    fi
    
    # Create .env if it doesn't exist
    local env_file="${PROJECT_ROOT}/.env"
    if [[ ! -f "$env_file" ]]; then
        cp "$env_example" "$env_file"
        warning "Created .env file from example - please customize it"
    fi
}

# Install development tools
install_dev_tools() {
    log "Installing development tools..."
    
    # Detect platform
    local platform=""
    case "$(uname -s)" in
        Darwin*) platform="macos" ;;
        Linux*)  platform="linux" ;;
        CYGWIN*|MINGW*) platform="windows" ;;
        *) platform="unknown" ;;
    esac
    
    case "$platform" in
        macos)
            # Check for Homebrew
            if ! command_exists brew; then
                log "Installing Homebrew..."
                /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"
            fi
            
            # Install common development tools
            install_tool "jq" "brew install jq"
            install_tool "curl" "brew install curl"
            install_tool "git" "brew install git"
            ;;
        linux)
            # Detect package manager
            if command_exists apt; then
                install_tool "jq" "sudo apt update && sudo apt install -y jq"
                install_tool "curl" "sudo apt install -y curl"
                install_tool "git" "sudo apt install -y git"
            elif command_exists yum; then
                install_tool "jq" "sudo yum install -y jq"
                install_tool "curl" "sudo yum install -y curl"
                install_tool "git" "sudo yum install -y git"
            else
                warning "Unknown package manager, please install tools manually"
            fi
            ;;
        *)
            warning "Unknown platform: $platform"
            ;;
    esac
}

# Main setup function
main() {
    log "Starting development environment setup..."
    log "Project root: $PROJECT_ROOT"
    
    # Create log file
    touch "$LOG_FILE"
    
    # Run setup steps
    setup_directories
    setup_env_files
    install_dev_tools
    setup_git_hooks
    
    success "Development environment setup completed!"
    log "Log file: $LOG_FILE"
    
    # Show next steps
    echo ""
    echo "🎉 Setup complete! Next steps:"
    echo "   1. Review and customize .env file"
    echo "   2. Install language-specific dependencies"
    echo "   3. Run initial tests to verify setup"
    echo "   4. Check the setup log: $LOG_FILE"
}

# Show usage information
usage() {
    cat << EOF
Development Environment Setup Script

Usage: $0 [options]

Options:
    -h, --help      Show this help message
    --skip-tools    Skip installation of development tools
    --skip-hooks    Skip Git hooks setup

Examples:
    $0                  # Full setup
    $0 --skip-tools     # Setup without installing tools
    $0 --skip-hooks     # Setup without Git hooks

EOF
}

# Parse command line arguments
SKIP_TOOLS=false
SKIP_HOOKS=false

while [[ $# -gt 0 ]]; do
    case $1 in
        -h|--help)
            usage
            exit 0
            ;;
        --skip-tools)
            SKIP_TOOLS=true
            shift
            ;;
        --skip-hooks)
            SKIP_HOOKS=true
            shift
            ;;
        *)
            error "Unknown option: $1"
            usage
            exit 1
            ;;
    esac
done

# Run main function
main