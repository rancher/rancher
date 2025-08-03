#!/bin/bash

# Scale Cluster Agent Setup Script
# This script helps set up the scale cluster agent for first-time users

set -e

echo "=== Scale Cluster Agent Setup ==="
echo

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Function to print colored output
print_status() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

print_header() {
    echo -e "${BLUE}$1${NC}"
}

# Check if running from the correct directory
if [ ! -f "main.go" ]; then
    print_error "This script must be run from the scale-cluster-agent directory"
    exit 1
fi

# Check if Go is installed
if ! command -v go &> /dev/null; then
    print_error "Go is not installed. Please install Go 1.23 or later."
    exit 1
fi

print_header "Step 1: Building the agent"
print_status "Building scale cluster agent..."

# Build the agent
if make scale-cluster-agent; then
    print_status "Agent built successfully!"
else
    print_error "Failed to build agent"
    exit 1
fi

print_header "Step 2: Setting up configuration"
print_status "Creating configuration directory..."

# Create configuration directory
CONFIG_DIR="$HOME/.scale-cluster-agent/config"
mkdir -p "$CONFIG_DIR"

# Check if config file already exists
if [ -f "$CONFIG_DIR/config" ]; then
    print_warning "Configuration file already exists at $CONFIG_DIR/config"
    read -p "Do you want to overwrite it? (y/N): " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        print_status "Keeping existing configuration"
    else
        print_status "Backing up existing configuration..."
        cp "$CONFIG_DIR/config" "$CONFIG_DIR/config.backup.$(date +%Y%m%d_%H%M%S)"
    fi
fi

# Create configuration file
print_status "Creating configuration file..."

cat > "$CONFIG_DIR/config" << 'EOF'
# Scale Cluster Agent Configuration
# Please update these values with your Rancher server information

# Rancher server URL (required)
RancherURL: "https://your-rancher-server.com/"

# Bearer token for Rancher API access (required)
# Get this from Rancher UI: User -> API & Keys -> Add Key
BearerToken: "token-xxxxx:xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"

# HTTP server port for REST API (optional, default: 9090)
ListenPort: 9090

# Log level: debug, info, warn, error (optional, default: info)
LogLevel: "info"
EOF

print_status "Configuration file created at $CONFIG_DIR/config"

# Copy cluster template if it doesn't exist
if [ ! -f "$CONFIG_DIR/cluster.yaml" ]; then
    print_status "Creating cluster template..."
    cp cluster.yaml "$CONFIG_DIR/cluster.yaml"
    print_status "Cluster template created at $CONFIG_DIR/cluster.yaml"
else
    print_warning "Cluster template already exists at $CONFIG_DIR/cluster.yaml"
fi

print_header "Step 3: Configuration Instructions"
echo
print_status "Please edit the configuration file with your Rancher server details:"
echo "  $CONFIG_DIR/config"
echo
echo "Required changes:"
echo "  1. Set RancherURL to your Rancher server URL"
echo "  2. Set BearerToken to your Rancher API token"
echo
echo "Optional changes:"
echo "  3. Adjust ListenPort if needed (default: 9090)"
echo "  4. Set LogLevel to 'debug' for more verbose output"
echo

print_header "Step 4: Getting Rancher API Token"
echo
echo "To get your Rancher API token:"
echo "  1. Log into your Rancher server"
echo "  2. Click on your user profile (top right)"
echo "  3. Go to 'API & Keys'"
echo "  4. Click 'Add Key'"
echo "  5. Give it a name (e.g., 'scale-agent')"
echo "  6. Select 'Read/Write' scope"
echo "  7. Copy the generated token"
echo "  8. Paste it in the BearerToken field in your config file"
echo

print_header "Step 5: Running the Agent"
echo
print_status "Once configured, you can start the agent with:"
echo "  ./bin/scale-cluster-agent"
echo
print_status "Or run it in the background:"
echo "  nohup ./bin/scale-cluster-agent > agent.log 2>&1 &"
echo

print_header "Step 6: Testing the Agent"
echo
print_status "You can test the agent using the provided test script:"
echo "  ./test-script.sh"
echo

print_header "Step 7: API Usage Examples"
echo
echo "Create a cluster:"
echo "  curl -X POST http://localhost:9090/clusters \\"
echo "    -H 'Content-Type: application/json' \\"
echo "    -d '{\"name\": \"test-cluster-001\"}'"
echo
echo "List clusters:"
echo "  curl http://localhost:9090/clusters"
echo
echo "Check health:"
echo "  curl http://localhost:9090/health"
echo
echo "Delete a cluster:"
echo "  curl -X DELETE http://localhost:9090/clusters/test-cluster-001"
echo

print_header "Setup Complete!"
echo
print_status "The scale cluster agent is now set up and ready to use."
print_status "Remember to update the configuration file with your Rancher server details."
echo
print_warning "Important: The agent will not work until you configure the RancherURL and BearerToken."
echo 