#!/bin/bash

# Function to parse YAML and set environment variables
parse_yaml() {
    local yaml_file="$1"
    
    # Check if file exists
    if [[ ! -f "$yaml_file" ]]; then
        echo "Error: File '$yaml_file' not found!" >&2
        return 1
    fi
    
    # Read the YAML file line by line
    while IFS= read -r line; do
        # Skip empty lines and comments
        [[ -z "$line" || "$line" =~ ^[[:space:]]*# ]] && continue
        
        # Parse key-value pairs (simple YAML format: key: value)
        if [[ "$line" =~ ^[[:space:]]*([^:]+):[[:space:]]*(.*)$ ]]; then
            key="${BASH_REMATCH[1]}"
            value="${BASH_REMATCH[2]}"
            
            # Trim whitespace from key and value
            key=$(echo "$key" | sed 's/^[[:space:]]*//;s/[[:space:]]*$//')
            value=$(echo "$value" | sed 's/^[[:space:]]*//;s/[[:space:]]*$//')
            
            # Remove quotes from value if present
            value=$(echo "$value" | sed 's/^["'\'']*//;s/["'\'']*$//')
            
            # Convert key to uppercase and replace spaces/dashes with underscores
            env_key=$(echo "$key" | tr '[:lower:]' '[:upper:]' | tr ' -' '_')
            
            # Set the environment variable
            export "$env_key"="$value"
            echo "Set $env_key=$value"
        fi
    done < "$yaml_file"
}

# Main script
main() {
    # Check if filename is provided
    if [[ $# -eq 0 ]]; then
        echo "Usage: $0 <yaml_file>"
        echo "Example: $0 config.yaml"
        exit 1
    fi
    
    yaml_file="$1"
    
    echo "Reading YAML file: $yaml_file"
    echo "Setting environment variables..."
    echo
    
    parse_yaml "$yaml_file"
    
    echo
    echo "Environment variables set successfully!"
    echo "Note: Variables are only available in this shell session."
    echo "To make them permanent, source this script: source $0 $yaml_file"
}

# Run main function with all arguments
main "$@"
