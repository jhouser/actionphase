#!/bin/bash
set -e

# Post-tool-use hook that tracks edited files and their repos
# This runs after Edit, MultiEdit, or Write tools complete successfully


# Read tool information from stdin
tool_info=$(cat)


# Extract relevant data
tool_name=$(echo "$tool_info" | jq -r '.tool_name // empty')
file_path=$(echo "$tool_info" | jq -r '.tool_input.file_path // empty')
session_id=$(echo "$tool_info" | jq -r '.session_id // empty')


# Skip if not an edit tool or no file path
if [[ ! "$tool_name" =~ ^(Edit|MultiEdit|Write)$ ]] || [[ -z "$file_path" ]]; then
    exit 0  # Exit 0 for skip conditions
fi

# Skip markdown files
if [[ "$file_path" =~ \.(md|markdown)$ ]]; then
    exit 0  # Exit 0 for skip conditions
fi

# Create cache directory in project
cache_dir="$CLAUDE_PROJECT_DIR/.claude/tsc-cache/${session_id:-default}"
mkdir -p "$cache_dir"

# Function to detect repo from file path
detect_repo() {
    local file="$1"
    local project_root="$CLAUDE_PROJECT_DIR"

    # Remove project root from path
    local relative_path="${file#$project_root/}"

    # Extract first directory component
    local repo=$(echo "$relative_path" | cut -d'/' -f1)

    # ActionPhase project structure
    case "$repo" in
        # Frontend (React/TypeScript)
        frontend)
            echo "frontend"
            ;;
        # Backend (Go)
        backend)
            echo "backend"
            ;;
        # Docs directory
        docs)
            echo "docs"
            ;;
        # E2E tests are in frontend
        e2e)
            echo "frontend"  # E2E tests are part of frontend
            ;;
        *)
            # Check if it's a file in root (justfile, etc.)
            if [[ ! "$relative_path" =~ / ]]; then
                echo "root"
            else
                # Default to backend for other paths
                echo "backend"
            fi
            ;;
    esac
}

# Function to get build command for repo
get_build_command() {
    local repo="$1"
    local project_root="$CLAUDE_PROJECT_DIR"

    case "$repo" in
        frontend)
            # Run through the container; host node_modules is stale/unmanaged.
            echo "just build-frontend"
            ;;
        backend)
            echo "just test"
            ;;
        *)
            echo ""
            ;;
    esac
}

# Function to get TSC command for repo
get_tsc_command() {
    local repo="$1"
    local project_root="$CLAUDE_PROJECT_DIR"

    case "$repo" in
        frontend)
            # Use `just type-check` (which runs `tsc -b --force` in the frontend
            # container). Never `tsc --noEmit` against frontend/tsconfig.json:
            # it is a solution-style file ("files": [] + project references), so
            # --noEmit checks ZERO files and exits 0 even with type errors.
            echo "just type-check"
            ;;
        backend)
            # Backend is Go, no TypeScript
            echo ""
            ;;
        *)
            echo ""
            ;;
    esac
}

# Detect repo
repo=$(detect_repo "$file_path")

# Skip if unknown repo
if [[ "$repo" == "unknown" ]] || [[ -z "$repo" ]]; then
    exit 0  # Exit 0 for skip conditions
fi

# Log edited file
echo "$(date +%s):$file_path:$repo" >> "$cache_dir/edited-files.log"

# Update affected repos list
if ! grep -q "^$repo$" "$cache_dir/affected-repos.txt" 2>/dev/null; then
    echo "$repo" >> "$cache_dir/affected-repos.txt"
fi

# Store build commands
build_cmd=$(get_build_command "$repo")
tsc_cmd=$(get_tsc_command "$repo")

if [[ -n "$build_cmd" ]]; then
    echo "$repo:build:$build_cmd" >> "$cache_dir/commands.txt.tmp"
fi

if [[ -n "$tsc_cmd" ]]; then
    echo "$repo:tsc:$tsc_cmd" >> "$cache_dir/commands.txt.tmp"
fi

# Remove duplicates from commands
if [[ -f "$cache_dir/commands.txt.tmp" ]]; then
    sort -u "$cache_dir/commands.txt.tmp" > "$cache_dir/commands.txt"
    rm -f "$cache_dir/commands.txt.tmp"
fi

# Exit cleanly
exit 0
