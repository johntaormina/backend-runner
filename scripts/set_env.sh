#!/bin/bash

# Usage: ./yaml_to_env.sh config.yaml
# or: source ./yaml_to_env.sh config.yaml (to set vars in current shell)

YAML_FILE="${1:-config.yaml}"

# Check if file exists
if [[ ! -f "$YAML_FILE" ]]; then
    echo "Error: File '$YAML_FILE' not found"
    exit 1
fi

# Check if yq is installed
if ! command -v yq &> /dev/null; then
    echo "Error: yq is not installed. Install it with:"
    echo "  brew install yq    # macOS"
    echo "  sudo apt install yq # Ubuntu/Debian"
    exit 1
fi

# Read YAML and set environment variables
# This handles flat key-value pairs
while IFS='=' read -r key value; do
    if [[ -n "$key" && -n "$value" ]]; then
        # Convert key to uppercase and replace dots/dashes with underscores
        env_key=$(echo "$key" | tr '[:lower:]' '[:upper:]' | tr '.-' '_')
        export "$env_key"="$value"
        echo "Set $env_key=$value"
    fi
done < <(yq eval '. as $item ireduce ({}; . * $item) | to_entries | .[] | .key + "=" + (.value | tostring)' "$YAML_FILE")

echo "Environment variables set from $YAML_FILE"
