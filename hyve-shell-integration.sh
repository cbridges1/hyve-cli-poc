#!/bin/bash
# Hyve Shell Integration
# Source this file in your ~/.bashrc or ~/.zshrc to enable seamless kubeconfig switching

# Find the hyve binary (adjust path as needed)
HYVE_BINARY=""
if [ -f "./hyve" ]; then
    HYVE_BINARY="./hyve"
elif command -v hyve >/dev/null 2>&1; then
    HYVE_BINARY="hyve"
else
    echo "Warning: hyve binary not found. Please ensure it's in your PATH or run from the hyve directory."
    return 1
fi

# Function to use a cluster's kubeconfig
hyve-use() {
    if [ $# -eq 0 ]; then
        echo "Usage: hyve-use <cluster-name>"
        echo "Available clusters:"
        $HYVE_BINARY kubeconfig list
        return 1
    fi
    
    local cluster_name="$1"
    
    # Run hyve use command and capture output
    local result
    result=$($HYVE_BINARY use "$cluster_name" 2>&1)
    
    if [ $? -eq 0 ]; then
        # Extract the export command from the output
        eval "$result"
        echo "✅ Successfully switched to cluster: $cluster_name"
        echo "💡 Current KUBECONFIG: $KUBECONFIG"
        echo "💡 To revert: hyve-unset"
    else
        echo "❌ Failed to switch to cluster: $cluster_name"
        echo "$result"
        return 1
    fi
}

# Function to unset kubeconfig
hyve-unset() {
    if [ -n "$KUBECONFIG" ]; then
        echo "💡 Unsetting KUBECONFIG (was: $KUBECONFIG)"
        unset KUBECONFIG
        echo "✅ Reverted to default kubectl configuration"
    else
        echo "💡 KUBECONFIG is not currently set"
    fi
}

# Function to show current kubeconfig status
hyve-status() {
    if [ -n "$KUBECONFIG" ]; then
        echo "📋 Current KUBECONFIG: $KUBECONFIG"
        if command -v kubectl >/dev/null 2>&1; then
            echo "🔍 Current context: $(kubectl config current-context 2>/dev/null || echo 'Unable to determine')"
        fi
    else
        echo "📋 Using default kubectl configuration"
        if command -v kubectl >/dev/null 2>&1; then
            echo "🔍 Current context: $(kubectl config current-context 2>/dev/null || echo 'Unable to determine')"
        fi
    fi
}

# Function to list available clusters
hyve-list() {
    $HYVE_BINARY kubeconfig list
}

# Function to sync kubeconfigs
hyve-sync() {
    echo "🔄 Syncing kubeconfigs from active clusters..."
    $HYVE_BINARY kubeconfig sync
}

# Auto-completion for hyve-use function
if [ -n "$BASH_VERSION" ] && command -v complete >/dev/null 2>&1; then
    _hyve_use_completion() {
        local cur="${COMP_WORDS[COMP_CWORD]}"
        local clusters
        clusters=$($HYVE_BINARY kubeconfig list 2>/dev/null | grep -E "^  [a-zA-Z]" | awk '{print $1}' 2>/dev/null)
        COMPREPLY=($(compgen -W "$clusters" -- "$cur"))
    }
    complete -F _hyve_use_completion hyve-use
elif [ -n "$ZSH_VERSION" ]; then
    # ZSH completion
    autoload -U compinit && compinit
    _hyve_use() {
        local clusters
        clusters=($($HYVE_BINARY kubeconfig list 2>/dev/null | grep -E "^  [a-zA-Z]" | awk '{print $1}' 2>/dev/null))
        _describe 'clusters' clusters
    }
    compdef _hyve_use hyve-use 2>/dev/null || true
fi

echo "🚀 Hyve shell integration loaded!"
echo "📖 Available commands:"
echo "   hyve-use <cluster>   # Switch to cluster"
echo "   hyve-unset          # Revert to default kubeconfig"
echo "   hyve-status         # Show current kubeconfig status"
echo "   hyve-list           # List available clusters"
echo "   hyve-sync           # Sync kubeconfigs from clusters"
echo ""
echo "💡 Example: hyve-use production"