#!/bin/bash
#
# Zeus Linux Development Environment Script
#
# This script manages the Docker-based Linux development environment for Zeus.
#
# Usage:
#   ./dev.sh           - Start interactive shell (builds image if needed)
#   ./dev.sh build     - Force rebuild the Docker image
#   ./dev.sh shell     - Start a new shell in existing container
#   ./dev.sh stop      - Stop the running container
#   ./dev.sh clean     - Remove container and image
#   ./dev.sh exec CMD  - Execute a command in the container
#
# Examples:
#   ./dev.sh                        # Start development shell
#   ./dev.sh exec make test         # Run tests
#   ./dev.sh exec make run file=main # Run playground/main.zs

set -e

# Configuration
IMAGE_NAME="zeus-dev"
CONTAINER_NAME="zeus-dev-container"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
NC='\033[0m' # No Color

print_header() {
    echo -e "${PURPLE}"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo "⚡ Zeus Linux Development Environment"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo -e "${NC}"
}

log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check if Docker is running
check_docker() {
    if ! docker info > /dev/null 2>&1; then
        log_error "Docker is not running. Please start Docker and try again."
        exit 1
    fi
}

# Check if image exists
image_exists() {
    docker image inspect "$IMAGE_NAME" > /dev/null 2>&1
}

# Check if container exists
container_exists() {
    docker container inspect "$CONTAINER_NAME" > /dev/null 2>&1
}

# Check if container is running
container_running() {
    [ "$(docker container inspect -f '{{.State.Running}}' "$CONTAINER_NAME" 2>/dev/null)" = "true" ]
}

# Build the Docker image
build_image() {
    log_info "Building Docker image '$IMAGE_NAME' for linux/amd64..."
    docker build --platform linux/amd64 -t "$IMAGE_NAME" "$SCRIPT_DIR"
    log_success "Image built successfully!"
}

# Start a new container
start_container() {
    if container_exists; then
        if container_running; then
            log_info "Container is already running."
            return
        else
            log_info "Starting existing container..."
            docker start "$CONTAINER_NAME"
            return
        fi
    fi

    log_info "Creating and starting new container..."
    docker run -d \
        --name "$CONTAINER_NAME" \
        -v "$SCRIPT_DIR:/zeus" \
        -w /zeus \
        --platform linux/amd64 \
        "$IMAGE_NAME" \
        tail -f /dev/null
    
    log_success "Container started!"
}

# Open interactive shell
open_shell() {
    log_info "Opening interactive shell..."
    docker exec -it "$CONTAINER_NAME" /bin/bash
}

# Stop the container
stop_container() {
    if container_running; then
        log_info "Stopping container..."
        docker stop "$CONTAINER_NAME"
        log_success "Container stopped."
    else
        log_warn "Container is not running."
    fi
}

# Remove container and optionally the image
clean() {
    if container_exists; then
        log_info "Removing container..."
        docker rm -f "$CONTAINER_NAME"
    fi
    
    if image_exists; then
        read -p "Also remove the Docker image? [y/N] " -n 1 -r
        echo
        if [[ $REPLY =~ ^[Yy]$ ]]; then
            log_info "Removing image..."
            docker rmi "$IMAGE_NAME"
        fi
    fi
    
    log_success "Cleanup complete."
}

# Execute a command in the container
exec_cmd() {
    if ! container_running; then
        log_error "Container is not running. Start it first with './dev.sh'"
        exit 1
    fi
    
    docker exec -it "$CONTAINER_NAME" "$@"
}

# Main script logic
main() {
    check_docker
    
    case "${1:-}" in
        build)
            print_header
            build_image
            ;;
        shell)
            if ! container_running; then
                log_error "Container is not running. Start it first with './dev.sh'"
                exit 1
            fi
            open_shell
            ;;
        stop)
            print_header
            stop_container
            ;;
        clean)
            print_header
            clean
            ;;
        exec)
            shift
            exec_cmd "$@"
            ;;
        help|--help|-h)
            print_header
            echo "Usage: ./dev.sh [command]"
            echo ""
            echo "Commands:"
            echo "  (none)    Start interactive shell (builds image if needed)"
            echo "  build     Force rebuild the Docker image"
            echo "  shell     Open a new shell in running container"
            echo "  stop      Stop the running container"
            echo "  clean     Remove container and image"
            echo "  exec CMD  Execute a command in the container"
            echo ""
            echo "Examples:"
            echo "  ./dev.sh                          # Start development shell"
            echo "  ./dev.sh exec make test           # Run tests"
            echo "  ./dev.sh exec make run file=main  # Run playground/main.zs"
            ;;
        "")
            print_header
            
            # Build image if it doesn't exist
            if ! image_exists; then
                build_image
            fi
            
            # Start container if needed
            start_container
            
            # Open shell
            open_shell
            ;;
        *)
            log_error "Unknown command: $1"
            echo "Run './dev.sh help' for usage information."
            exit 1
            ;;
    esac
}

main "$@"

