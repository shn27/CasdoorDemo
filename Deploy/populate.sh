#!/bin/bash

set -euo pipefail

# Configuration
ENV_FILE="${ENV_FILE:-.env}"
TEMPLATE_FILE="${TEMPLATE_FILE:-init_data.json.tmpl}"
OUTPUT_FILE="${OUTPUT_FILE:-init_data.json}"

# Color codes for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Logging functions
log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1" >&2
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

# Check if required files exist
check_files() {
    if [[ ! -f "$TEMPLATE_FILE" ]]; then
        log_error "Template file '$TEMPLATE_FILE' not found"
        exit 1
    fi

    if [[ ! -f "$ENV_FILE" ]]; then
        log_error "Environment file '$ENV_FILE' not found"
        exit 1
    fi
}

# Load environment variables from .env file
load_env() {
    log_info "Loading environment variables from '$ENV_FILE'"

    # Export variables from .env file
    # Skip comments and empty lines
    while IFS= read -r line || [[ -n "$line" ]]; do
        # Skip comments and empty lines
        [[ "$line" =~ ^[[:space:]]*# ]] && continue
        [[ -z "${line// }" ]] && continue

        # Export the variable
        if [[ "$line" =~ ^[[:space:]]*([A-Za-z_][A-Za-z0-9_]*)[[:space:]]*=[[:space:]]*(.*)$ ]]; then
            var_name="${BASH_REMATCH[1]}"
            var_value="${BASH_REMATCH[2]}"

            # Remove surrounding quotes if present
            var_value="${var_value%\"}"
            var_value="${var_value#\"}"
            var_value="${var_value%\'}"
            var_value="${var_value#\'}"

            export "$var_name=$var_value"
        fi
    done < "$ENV_FILE"
}

# Expand placeholders in the template
# Supports:
# - ${VAR} -> replaced with environment variable value
# - ${VAR:default} -> replaced with env var or default if not set
expand_placeholders() {
    local content="$1"
    local missing_vars=()

    # Find all placeholders in the format ${VAR} or ${VAR:default}
    while [[ "$content" =~ \$\{([^}]+)\} ]]; do
        local placeholder="${BASH_REMATCH[1]}"
        local var_name="$placeholder"
        local default_value=""
        local has_default=false

        # Check if placeholder has a default value (format: VAR:default)
        if [[ "$placeholder" =~ ^([^:]+):(.*)$ ]]; then
            var_name="${BASH_REMATCH[1]}"
            default_value="${BASH_REMATCH[2]}"
            has_default=true
        fi

        # Get the variable value
        local var_value="${!var_name:-}"

        if [[ -n "$var_value" ]]; then
            # Replace placeholder with variable value
            content="${content//\$\{$placeholder\}/$var_value}"
        elif [[ "$has_default" == true ]]; then
            # Use default value
            content="${content//\$\{$placeholder\}/$default_value}"
        else
            # Variable not found and no default
            missing_vars+=("$var_name")
            # Remove the placeholder to avoid infinite loop
            content="${content//\$\{$placeholder\}/}"
        fi
    done

    # Report missing variables
    if [[ ${#missing_vars[@]} -gt 0 ]]; then
        log_error "Environment variables [${missing_vars[*]}] not found and don't have any default"
        return 1
    fi

    echo "$content"
}

# Main function
main() {
    log_info "Starting template population process"

    # Check required files
    check_files

    # Load environment variables
    load_env

    # Read template file
    log_info "Reading template file '$TEMPLATE_FILE'"
    template_content=$(cat "$TEMPLATE_FILE")

    # Expand placeholders
    log_info "Expanding placeholders"
    expanded_content=$(expand_placeholders "$template_content")

    if [[ $? -ne 0 ]]; then
        log_error "Failed to expand placeholders"
        exit 1
    fi

    # Write output file
    log_info "Writing output to '$OUTPUT_FILE'"
    echo "$expanded_content" > "$OUTPUT_FILE"

    log_info "Successfully created '$OUTPUT_FILE' from '$TEMPLATE_FILE'"
}

# Run main function
main "$@"