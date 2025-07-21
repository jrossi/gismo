#!/bin/bash
# Smart Git Commit Script
# Automatically generates meaningful commit messages and handles common workflows

set -euo pipefail

# Configuration
readonly SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly PROJECT_ROOT="$(git rev-parse --show-toplevel 2>/dev/null || pwd)"

# Colors for output
readonly RED='\033[0;31m'
readonly GREEN='\033[0;32m'
readonly YELLOW='\033[1;33m'
readonly BLUE='\033[0;34m'
readonly CYAN='\033[0;36m'
readonly NC='\033[0m' # No Color

# Logging functions
info() {
    echo -e "${BLUE}ℹ${NC} $*"
}

success() {
    echo -e "${GREEN}✓${NC} $*"
}

warning() {
    echo -e "${YELLOW}⚠${NC} $*"
}

error() {
    echo -e "${RED}✗${NC} $*"
    exit 1
}

# Check if we're in a git repository
check_git_repo() {
    if ! git rev-parse --git-dir > /dev/null 2>&1; then
        error "Not in a git repository"
    fi
}

# Get current branch name
get_current_branch() {
    git branch --show-current
}

# Check if there are staged changes
has_staged_changes() {
    ! git diff --cached --quiet
}

# Check if there are unstaged changes
has_unstaged_changes() {
    ! git diff --quiet
}

# Stage all changes interactively
stage_changes() {
    if has_unstaged_changes; then
        echo -e "${CYAN}Unstaged changes detected:${NC}"
        git status --short
        echo ""
        
        read -p "Stage all changes? (y/n/interactive): " -r choice
        case "$choice" in
            y|Y)
                git add .
                success "All changes staged"
                ;;
            i|I|interactive)
                git add --interactive
                ;;
            n|N)
                warning "No changes staged"
                ;;
            *)
                warning "Invalid choice, staging all changes"
                git add .
                ;;
        esac
    fi
}

# Analyze changes to generate commit type
analyze_changes() {
    local files_changed
    files_changed=$(git diff --cached --name-only)
    
    if [[ -z "$files_changed" ]]; then
        echo "none"
        return
    fi
    
    # Check for different types of changes
    local has_new_files=false
    local has_tests=false
    local has_docs=false
    local has_config=false
    local has_dependencies=false
    
    while IFS= read -r file; do
        case "$file" in
            *_test.* | *test* | *spec*)
                has_tests=true
                ;;
            *.md | *README* | *CHANGELOG* | docs/*)
                has_docs=true
                ;;
            *.json | *.yaml | *.yml | *.toml | *.ini | *config*)
                has_config=true
                ;;
            *package*.json | *go.mod | *requirements*.txt | *Cargo.toml)
                has_dependencies=true
                ;;
        esac
        
        if git diff --cached --diff-filter=A --name-only | grep -q "^$file$"; then
            has_new_files=true
        fi
    done <<< "$files_changed"
    
    # Determine commit type based on analysis
    if [[ "$has_dependencies" == true ]]; then
        echo "deps"
    elif [[ "$has_tests" == true ]]; then
        echo "test"
    elif [[ "$has_docs" == true ]]; then
        echo "docs"
    elif [[ "$has_config" == true ]]; then
        echo "config"
    elif [[ "$has_new_files" == true ]]; then
        echo "feat"
    else
        echo "fix"
    fi
}

# Generate commit message based on changes
generate_commit_message() {
    local commit_type="$1"
    local files_changed
    files_changed=$(git diff --cached --name-only | wc -l | tr -d ' ')
    
    # Get first few changed files for context
    local file_list
    file_list=$(git diff --cached --name-only | head -3 | tr '\n' ' ')
    
    case "$commit_type" in
        feat)
            echo "feat: add new functionality"
            ;;
        fix)
            echo "fix: resolve issues in $file_list"
            ;;
        docs)
            echo "docs: update documentation"
            ;;
        test)
            echo "test: add/update test cases"
            ;;
        config)
            echo "config: update configuration files"
            ;;
        deps)
            echo "deps: update dependencies"
            ;;
        refactor)
            echo "refactor: improve code structure"
            ;;
        style)
            echo "style: format code and fix styling"
            ;;
        *)
            echo "update: modify $files_changed file(s)"
            ;;
    esac
}

# Show commit preview
show_commit_preview() {
    echo -e "${CYAN}Commit preview:${NC}"
    echo "Branch: $(get_current_branch)"
    echo "Files changed: $(git diff --cached --name-only | wc -l | tr -d ' ')"
    echo ""
    echo "Files to be committed:"
    git diff --cached --name-status | sed 's/^/  /'
    echo ""
}

# Interactive commit message editing
edit_commit_message() {
    local suggested_message="$1"
    
    echo "Suggested commit message: $suggested_message"
    echo ""
    
    read -p "Use this message? (y/n/edit): " -r choice
    case "$choice" in
        y|Y|"")
            echo "$suggested_message"
            ;;
        e|E|edit)
            read -p "Enter commit message: " -r custom_message
            echo "$custom_message"
            ;;
        n|N)
            read -p "Enter commit message: " -r custom_message
            echo "$custom_message"
            ;;
        *)
            echo "$suggested_message"
            ;;
    esac
}

# Perform the commit
do_commit() {
    local message="$1"
    local push_after="$2"
    
    info "Committing changes..."
    
    if git commit -m "$message"; then
        local commit_hash
        commit_hash=$(git rev-parse --short HEAD)
        success "Committed successfully: $commit_hash"
        
        if [[ "$push_after" == true ]]; then
            info "Pushing to remote..."
            local current_branch
            current_branch=$(get_current_branch)
            
            if git push origin "$current_branch"; then
                success "Pushed to origin/$current_branch"
            else
                warning "Failed to push, you may need to pull first"
            fi
        fi
    else
        error "Commit failed"
    fi
}

# Main workflow
main() {
    local auto_message=false
    local push_after=false
    local commit_type=""
    local custom_message=""
    
    # Parse command line arguments
    while [[ $# -gt 0 ]]; do
        case $1 in
            -a|--auto)
                auto_message=true
                shift
                ;;
            -p|--push)
                push_after=true
                shift
                ;;
            -t|--type)
                commit_type="$2"
                shift 2
                ;;
            -m|--message)
                custom_message="$2"
                shift 2
                ;;
            -h|--help)
                usage
                exit 0
                ;;
            *)
                error "Unknown option: $1"
                ;;
        esac
    done
    
    # Check prerequisites
    check_git_repo
    
    # Handle staging
    if ! has_staged_changes; then
        if has_unstaged_changes; then
            stage_changes
        else
            warning "No changes to commit"
            exit 0
        fi
    fi
    
    # Verify we have changes to commit
    if ! has_staged_changes; then
        warning "No staged changes to commit"
        exit 0
    fi
    
    # Show what will be committed
    show_commit_preview
    
    # Generate or use commit message
    local final_message
    if [[ -n "$custom_message" ]]; then
        final_message="$custom_message"
    elif [[ "$auto_message" == true ]]; then
        local detected_type
        detected_type=$(analyze_changes)
        final_message=$(generate_commit_message "$detected_type")
        info "Auto-generated message: $final_message"
    else
        local suggested_type="${commit_type:-$(analyze_changes)}"
        local suggested_message
        suggested_message=$(generate_commit_message "$suggested_type")
        final_message=$(edit_commit_message "$suggested_message")
    fi
    
    # Perform the commit
    do_commit "$final_message" "$push_after"
}

# Show usage information
usage() {
    cat << EOF
Smart Git Commit Script

Usage: $0 [options]

Options:
    -a, --auto          Use auto-generated commit message
    -p, --push          Push after successful commit
    -t, --type TYPE     Specify commit type (feat, fix, docs, test, etc.)
    -m, --message MSG   Use custom commit message
    -h, --help          Show this help message

Examples:
    $0                      # Interactive commit
    $0 --auto              # Auto-generated message
    $0 --auto --push       # Auto commit and push
    $0 -t feat -m "add new feature"
    $0 -m "custom commit message" --push

Commit Types:
    feat        New feature
    fix         Bug fix
    docs        Documentation changes
    test        Test changes
    config      Configuration changes
    deps        Dependency updates
    refactor    Code refactoring
    style       Code formatting

EOF
}

# Run main function with all arguments
main "$@"