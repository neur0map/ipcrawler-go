#!/bin/bash
# IPCrawler Production Installer
# https://github.com/neur0map/ipcrawler-go
# 
# This script installs IPCrawler globally with automatic dependency detection
# and system-specific configuration for both sudo and normal user execution.

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Installation variables
REPO_URL="https://github.com/neur0map/ipcrawler-go.git"
INSTALL_DIR="/opt/ipcrawler"
BINARY_PATH="/usr/local/bin/ipcrawler"
TEMP_DIR="/tmp/ipcrawler-install"
GO_MIN_VERSION="1.19"

# System detection
OS=""
DISTRO=""
PACKAGE_MANAGER=""
ARCH=""

print_banner() {
    echo -e "${PURPLE}"
    echo "╔══════════════════════════════════════════════════════════════╗"
    echo "║                    IPCrawler Installer                       ║"
    echo "║              Security Testing Tool Suite                     ║"
    echo "║                                                              ║"
    echo "║  Repository: https://github.com/neur0map/ipcrawler-go        ║"
    echo "╚══════════════════════════════════════════════════════════════╝"
    echo -e "${NC}"
}

log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

log_step() {
    echo -e "${CYAN}[STEP]${NC} $1"
}

# Check if running as root
check_root() {
    if [[ $EUID -eq 0 ]]; then
        log_warning "Running as root user"
        return 0
    else
        log_info "Running as non-root user (will use sudo when needed)"
        return 1
    fi
}

# Detect operating system and distribution
detect_system() {
    log_step "Detecting system architecture and OS..."
    
    # Detect architecture
    ARCH=$(uname -m)
    case $ARCH in
        x86_64|amd64)
            ARCH="amd64"
            ;;
        i386|i686)
            ARCH="386"
            ;;
        aarch64|arm64)
            ARCH="arm64"
            ;;
        armv7l)
            ARCH="arm"
            ;;
        *)
            log_error "Unsupported architecture: $ARCH"
            exit 1
            ;;
    esac
    
    # Detect OS
    case "$(uname -s)" in
        Linux*)
            OS="linux"
            detect_linux_distro
            ;;
        Darwin*)
            OS="darwin"
            DISTRO="macos"
            PACKAGE_MANAGER="brew"
            ;;
        CYGWIN*|MINGW*|MSYS*)
            OS="windows"
            DISTRO="windows"
            log_error "Windows installation not supported by this script"
            log_info "Please use WSL2 or install manually"
            exit 1
            ;;
        *)
            log_error "Unsupported operating system: $(uname -s)"
            exit 1
            ;;
    esac
    
    log_success "Detected: $OS $DISTRO ($ARCH)"
}

# Detect Linux distribution
detect_linux_distro() {
    if [[ -f /etc/os-release ]]; then
        . /etc/os-release
        DISTRO=$ID
    elif [[ -f /etc/debian_version ]]; then
        DISTRO="debian"
    elif [[ -f /etc/redhat-release ]]; then
        DISTRO="rhel"
    else
        DISTRO="unknown"
    fi
    
    # Set package manager based on distro
    case $DISTRO in
        ubuntu|debian|kali|parrot)
            PACKAGE_MANAGER="apt"
            ;;
        fedora|rhel|centos|rocky|almalinux)
            PACKAGE_MANAGER="dnf"
            ;;
        arch|manjaro|endeavouros)
            PACKAGE_MANAGER="pacman"
            ;;
        opensuse*)
            PACKAGE_MANAGER="zypper"
            ;;
        alpine)
            PACKAGE_MANAGER="apk"
            ;;
        *)
            PACKAGE_MANAGER="unknown"
            log_warning "Unknown package manager for distro: $DISTRO"
            ;;
    esac
}

# Check if command exists
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

# Install package using system package manager
install_package() {
    local package=$1
    local package_name=${2:-$package}
    
    if command_exists "$package"; then
        log_success "$package_name is already installed"
        return 0
    fi
    
    log_step "Installing $package_name..."
    
    case $PACKAGE_MANAGER in
        apt)
            sudo apt update >/dev/null 2>&1
            sudo apt install -y "$package"
            ;;
        dnf)
            sudo dnf install -y "$package"
            ;;
        pacman)
            sudo pacman -S --noconfirm "$package"
            ;;
        zypper)
            sudo zypper install -y "$package"
            ;;
        apk)
            sudo apk add "$package"
            ;;
        brew)
            brew install "$package"
            ;;
        *)
            log_error "Cannot install $package_name - unsupported package manager: $PACKAGE_MANAGER"
            log_info "Please install $package_name manually"
            return 1
            ;;
    esac
    
    if command_exists "$package"; then
        log_success "$package_name installed successfully"
        return 0
    else
        log_error "Failed to install $package_name"
        return 1
    fi
}

# Check and install Go
install_go() {
    if command_exists go; then
        local go_version=$(go version | grep -oE 'go[0-9]+\.[0-9]+' | sed 's/go//')
        local major=$(echo $go_version | cut -d. -f1)
        local minor=$(echo $go_version | cut -d. -f2)
        local min_major=$(echo $GO_MIN_VERSION | cut -d. -f1)
        local min_minor=$(echo $GO_MIN_VERSION | cut -d. -f2)
        
        if [[ $major -gt $min_major ]] || [[ $major -eq $min_major && $minor -ge $min_minor ]]; then
            log_success "Go $go_version is already installed (required: $GO_MIN_VERSION+)"
            return 0
        else
            log_warning "Go $go_version is installed but version $GO_MIN_VERSION+ is required"
        fi
    fi
    
    log_step "Installing Go $GO_MIN_VERSION+..."
    
    case $OS in
        linux)
            # Download and install Go from official source
            local go_version="1.21.3"
            local go_url="https://golang.org/dl/go${go_version}.${OS}-${ARCH}.tar.gz"
            local go_archive="/tmp/go${go_version}.${OS}-${ARCH}.tar.gz"
            
            log_info "Downloading Go from $go_url..."
            curl -fsSL "$go_url" -o "$go_archive"
            
            sudo rm -rf /usr/local/go
            sudo tar -C /usr/local -xzf "$go_archive"
            rm "$go_archive"
            
            # Add Go to PATH if not already there
            if ! echo "$PATH" | grep -q "/usr/local/go/bin"; then
                echo 'export PATH=$PATH:/usr/local/go/bin' | sudo tee -a /etc/profile >/dev/null
                export PATH=$PATH:/usr/local/go/bin
            fi
            ;;
        darwin)
            if command_exists brew; then
                brew install go
            else
                log_error "Homebrew not found. Please install Go manually from https://golang.org/dl/"
                exit 1
            fi
            ;;
    esac
    
    # Verify installation
    if command_exists go; then
        log_success "Go installed successfully: $(go version)"
    else
        log_error "Go installation failed"
        exit 1
    fi
}

# Install security tools dependencies
install_security_tools() {
    log_step "Installing security testing dependencies..."
    
    # Core networking tools
    case $PACKAGE_MANAGER in
        apt)
            sudo apt update >/dev/null 2>&1
            sudo apt install -y \
                nmap \
                dnsutils \
                curl \
                wget \
                git \
                make \
                build-essential
            ;;
        dnf)
            sudo dnf install -y \
                nmap \
                bind-utils \
                curl \
                wget \
                git \
                make \
                gcc \
                gcc-c++
            ;;
        pacman)
            sudo pacman -S --noconfirm \
                nmap \
                bind-tools \
                curl \
                wget \
                git \
                make \
                base-devel
            ;;
        zypper)
            sudo zypper install -y \
                nmap \
                bind-utils \
                curl \
                wget \
                git \
                make \
                gcc \
                gcc-c++
            ;;
        apk)
            sudo apk add \
                nmap \
                bind-tools \
                curl \
                wget \
                git \
                make \
                build-base
            ;;
        brew)
            brew install \
                nmap \
                bind \
                curl \
                wget \
                git \
                make
            ;;
    esac
    
    # Install Naabu (Go-based port scanner)
    if ! command_exists naabu; then
        log_step "Installing Naabu port scanner..."
        go install -v github.com/projectdiscovery/naabu/v2/cmd/naabu@latest
        
        # Add Go bin to PATH for current session
        export PATH=$PATH:$(go env GOPATH)/bin
    fi
    
    log_success "Security tools installation completed"
}

# Clone and build IPCrawler
build_ipcrawler() {
    log_step "Cloning IPCrawler repository..."
    
    # Clean up any existing installation
    sudo rm -rf "$TEMP_DIR"
    mkdir -p "$TEMP_DIR"
    
    # Clone repository
    git clone "$REPO_URL" "$TEMP_DIR"
    cd "$TEMP_DIR"
    
    log_step "Building IPCrawler..."
    
    # Install Go dependencies
    go mod tidy
    
    # Build the application
    make build
    
    if [[ ! -f "bin/ipcrawler" ]]; then
        log_error "Build failed - binary not found"
        exit 1
    fi
    
    log_success "IPCrawler built successfully"
}

# Install IPCrawler globally
install_globally() {
    log_step "Installing IPCrawler globally..."
    
    # Create installation directory
    sudo mkdir -p "$INSTALL_DIR"
    
    # Copy binary and supporting files
    sudo cp -r "$TEMP_DIR"/* "$INSTALL_DIR/"
    sudo chmod +x "$INSTALL_DIR/bin/ipcrawler"
    
    # Create wrapper script that sets proper working directory
    local wrapper_script="$BINARY_PATH"
    sudo tee "$wrapper_script" >/dev/null <<EOF
#!/bin/bash
# IPCrawler Global Wrapper
# Sets correct working directory for workflow and config access

# Get the installation directory
INSTALL_DIR="$INSTALL_DIR"

# Change to installation directory to access workflows/, configs/, tools/
cd "\$INSTALL_DIR" || {
    echo "Error: Cannot access IPCrawler installation directory: \$INSTALL_DIR"
    exit 1
}

# Execute IPCrawler with all arguments passed through
exec "\$INSTALL_DIR/bin/ipcrawler" "\$@"
EOF
    
    sudo chmod +x "$wrapper_script"
    
    # Also create direct symlink for advanced users who want direct binary access
    sudo ln -s "$INSTALL_DIR/bin/ipcrawler" "/usr/local/bin/ipcrawler-direct"
    
    # Set proper permissions
    sudo chown -R root:root "$INSTALL_DIR"
    sudo chmod 755 "$wrapper_script"
    
    log_success "IPCrawler installed to $INSTALL_DIR"
    log_success "Global wrapper created: $BINARY_PATH"
    log_success "Direct binary link: /usr/local/bin/ipcrawler-direct"
}

# Configure for sudo and normal user execution
configure_execution() {
    log_step "Configuring execution permissions..."
    
    # Create sudoers rule for IPCrawler (allows running security tools with elevated privileges)
    local sudoers_file="/etc/sudoers.d/ipcrawler"
    
    if [[ ! -f "$sudoers_file" ]]; then
        log_info "Creating sudoers configuration for security tools..."
        sudo tee "$sudoers_file" >/dev/null <<EOF
# IPCrawler - Allow users to run security tools with elevated privileges
%sudo ALL=(ALL) NOPASSWD: /usr/local/bin/ipcrawler, /usr/local/bin/ipcrawler-direct
%wheel ALL=(ALL) NOPASSWD: /usr/local/bin/ipcrawler, /usr/local/bin/ipcrawler-direct
EOF
        sudo chmod 440 "$sudoers_file"
        log_success "Sudoers configuration created"
    fi
    
    # Create wrapper script for enhanced functionality
    local wrapper_script="/usr/local/bin/ipcrawler-sudo"
    sudo tee "$wrapper_script" >/dev/null <<'EOF'
#!/bin/bash
# IPCrawler sudo wrapper - preserves environment while running with elevated privileges

if [[ $EUID -eq 0 ]]; then
    # Already running as root
    exec /usr/local/bin/ipcrawler "$@"
else
    # Run with sudo, preserving necessary environment variables
    exec sudo -E /usr/local/bin/ipcrawler "$@"
fi
EOF
    sudo chmod +x "$wrapper_script"
    
    log_success "Execution configuration completed"
}

# Verify installation
verify_installation() {
    log_step "Verifying installation..."
    
    # Check if binary exists and is executable
    if [[ -x "$BINARY_PATH" ]]; then
        log_success "IPCrawler binary is executable"
    else
        log_error "IPCrawler binary is not executable"
        exit 1
    fi
    
    # Test basic functionality
    if "$BINARY_PATH" --version >/dev/null 2>&1; then
        log_success "IPCrawler responds to --version"
    else
        log_warning "IPCrawler --version test failed (this may be normal)"
    fi
    
    # Check if security tools are available
    local tools_status=""
    for tool in nmap nslookup naabu; do
        if command_exists "$tool"; then
            tools_status="${tools_status}✓ $tool "
        else
            tools_status="${tools_status}✗ $tool "
        fi
    done
    
    log_info "Security tools status: $tools_status"
}

# Cleanup temporary files
cleanup() {
    log_step "Cleaning up temporary files..."
    rm -rf "$TEMP_DIR"
    log_success "Cleanup completed"
}

# Show post-installation information
show_post_install() {
    echo -e "\n${GREEN}╔══════════════════════════════════════════════════════════════╗"
    echo -e "║                  Installation Complete!                     ║"
    echo -e "╚══════════════════════════════════════════════════════════════╝${NC}\n"
    
    echo -e "${CYAN}Quick Start:${NC}"
    echo -e "  ${YELLOW}ipcrawler <target>${NC}           # Run security scan"
    echo -e "  ${YELLOW}ipcrawler google.com${NC}         # Example scan"
    echo -e "  ${YELLOW}sudo ipcrawler <target>${NC}      # Run with elevated privileges"
    echo ""
    
    echo -e "${CYAN}Available Commands:${NC}"
    echo -e "  ${YELLOW}ipcrawler --help${NC}             # Show help"
    echo -e "  ${YELLOW}ipcrawler registry list${NC}      # List available tools"
    echo -e "  ${YELLOW}ipcrawler-sudo <target>${NC}      # Auto-sudo wrapper"
    echo -e "  ${YELLOW}ipcrawler-direct <target>${NC}    # Direct binary access"
    echo ""
    
    echo -e "${CYAN}Installation Details:${NC}"
    echo -e "  Wrapper Script: ${BINARY_PATH}"
    echo -e "  Direct Binary: /usr/local/bin/ipcrawler-direct"
    echo -e "  Install Directory: ${INSTALL_DIR}"
    echo -e "  Configuration: ${INSTALL_DIR}/configs/"
    echo -e "  Workflows: ${INSTALL_DIR}/workflows/"
    echo -e "  Tools: ${INSTALL_DIR}/tools/"
    echo -e "  Logs: ${INSTALL_DIR}/local_files/logs/"
    echo ""
    
    echo -e "${CYAN}Documentation:${NC}"
    echo -e "  Repository: ${REPO_URL}"
    echo -e "  Local Docs: ${INSTALL_DIR}/CLAUDE.md"
    echo ""
    
    echo -e "${GREEN}Happy hunting! 🎯${NC}"
}

# Main installation function
main() {
    print_banner
    
    # Check prerequisites
    log_step "Starting IPCrawler installation..."
    
    # Detect system
    detect_system
    
    # Check permissions
    if ! check_root && ! sudo -n true 2>/dev/null; then
        log_error "This script requires sudo privileges"
        log_info "Please run: sudo $0"
        exit 1
    fi
    
    # Install dependencies
    install_go
    install_security_tools
    
    # Build and install IPCrawler
    build_ipcrawler
    install_globally
    configure_execution
    
    # Verify and cleanup
    verify_installation
    cleanup
    
    # Show completion message
    show_post_install
}

# Handle interrupts gracefully
trap cleanup EXIT

# Error handling
set -e
trap 'log_error "Installation failed at line $LINENO"' ERR

# Run main installation
main "$@"