{ pkgs ? import <nixpkgs> {} }:

pkgs.mkShell {
  buildInputs = with pkgs; [
    # Go toolchain
    go
    
    # Task runner (go-task)
    go-task
    
    # Additional useful Go development tools
    gopls          # Go Language Server
    golangci-lint  # Go linter
    delve          # Go debugger
    gotools        # Additional Go tools (goimports, etc.)
    gofumpt
    
    # Git for version control
    git
    
    # Common development utilities
    curl
    jq
    wget
  ];

  shellHook = ''
    echo "🚀 Go development environment loaded!"
    echo ""
    echo "Available tools:"
    echo "  • Go $(go version | cut -d' ' -f3)"
    echo "  • Task $(task --version)"
    echo "  • gopls (Go Language Server)"
    echo "  • golangci-lint"
    echo "  • delve (Go debugger)"
    echo ""
    echo "Getting started:"
    echo "  • Initialize a new Go module: go mod init <module-name>"
    echo "  • Create a Taskfile.yml for task automation"
    echo "  • Run 'task --list' to see available tasks"
    echo ""
    
    # Set up Go environment variables
    export GOPATH="$HOME/go"
    export GOBIN="$GOPATH/bin"
    export PATH="$GOBIN:$PATH"
    
    # Create GOPATH directories if they don't exist
    mkdir -p "$GOPATH"/{bin,src,pkg}
    
    echo "Environment variables set:"
    echo "  • GOPATH=$GOPATH"
    echo "  • GOBIN=$GOBIN"
    echo ""
  '';

  # Set environment variables
  GOROOT = "${pkgs.go}/share/go";
}
