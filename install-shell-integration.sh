#!/bin/bash
# Hyve Shell Integration Installer
# This script installs shell functions for seamless kubeconfig switching

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
INTEGRATION_FILE="$SCRIPT_DIR/hyve-shell-integration.sh"

# Detect shell
detect_shell() {
    if [ -n "$ZSH_VERSION" ]; then
        echo "zsh"
    elif [ -n "$BASH_VERSION" ]; then
        echo "bash"
    else
        echo "unknown"
    fi
}

# Get shell config file
get_shell_config() {
    local shell_type="$1"
    case "$shell_type" in
        "bash")
            if [ -f "$HOME/.bashrc" ]; then
                echo "$HOME/.bashrc"
            else
                echo "$HOME/.bash_profile"
            fi
            ;;
        "zsh")
            echo "$HOME/.zshrc"
            ;;
        *)
            echo "$HOME/.profile"
            ;;
    esac
}

main() {
    echo "🚀 Hyve Shell Integration Installer"
    echo "=================================="
    echo ""
    
    # Check if hyve binary exists
    if [ ! -f "$SCRIPT_DIR/hyve" ]; then
        echo "❌ Error: hyve binary not found in $SCRIPT_DIR"
        echo "   Please run 'go build -o hyve .' first"
        exit 1
    fi
    
    # Check if integration file exists
    if [ ! -f "$INTEGRATION_FILE" ]; then
        echo "❌ Error: Integration file not found: $INTEGRATION_FILE"
        exit 1
    fi
    
    # Detect shell
    shell_type=$(detect_shell)
    shell_config=$(get_shell_config "$shell_type")
    
    echo "🔍 Detected shell: $shell_type"
    echo "📝 Shell config file: $shell_config"
    echo ""
    
    # Check if already installed
    if grep -q "hyve-shell-integration.sh" "$shell_config" 2>/dev/null; then
        echo "✅ Hyve shell integration already installed in $shell_config"
    else
        echo "📦 Installing shell integration..."
        
        # Add source line to shell config
        echo "" >> "$shell_config"
        echo "# Hyve cluster management shell integration" >> "$shell_config"
        echo "if [ -f \"$INTEGRATION_FILE\" ]; then" >> "$shell_config"
        echo "    source \"$INTEGRATION_FILE\"" >> "$shell_config"
        echo "fi" >> "$shell_config"
        
        echo "✅ Added integration to $shell_config"
    fi
    
    echo ""
    echo "🎉 Installation complete!"
    echo ""
    echo "📋 Next steps:"
    echo "   1. Restart your terminal or run: source $shell_config"
    echo "   2. Use: hyve-use <cluster-name> to switch clusters"
    echo ""
    echo "💡 Example usage:"
    echo "   hyve-list              # List available clusters"
    echo "   hyve-use production    # Switch to production cluster"
    echo "   hyve-status           # Check current cluster"
    echo "   hyve-unset            # Revert to default"
    echo ""
    
    # Offer to source immediately
    read -p "🔄 Would you like to reload your shell configuration now? (y/n): " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        echo "🔄 Reloading shell configuration..."
        if [ "$shell_type" = "zsh" ]; then
            exec zsh
        else
            exec bash
        fi
    else
        echo "💡 Remember to restart your terminal or run: source $shell_config"
    fi
}

main "$@"