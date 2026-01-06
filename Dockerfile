# Zeus Language - Linux Development Environment
# 
# This Dockerfile creates a complete development environment for Zeus compiler development.
# It includes Go, Zig, LLVM, and all required dependencies.
#
# Usage:
#   ./dev.sh         - Build image and start interactive shell
#   ./dev.sh build   - Rebuild the Docker image
#   ./dev.sh shell   - Start a shell in existing container

FROM ubuntu:24.04

LABEL maintainer="Zeus Team"
LABEL description="Zeus Language Linux Development Environment"

# Prevent interactive prompts during package installation
ENV DEBIAN_FRONTEND=noninteractive

# Set timezone
ENV TZ=UTC

# Install essential build tools and dependencies
RUN apt-get update && apt-get install -y \
    # Build essentials
    build-essential \
    make \
    git \
    curl \
    wget \
    # LLVM 18 and Clang
    llvm-18 \
    llvm-18-dev \
    clang-18 \
    lld-18 \
    # Runtime dependencies
    libunwind-dev \
    libgc-dev \
    # Useful development tools
    gdb \
    valgrind \
    strace \
    vim \
    nano \
    less \
    htop \
    # For LSP development/testing
    jq \
    # Clean up apt cache
    && rm -rf /var/lib/apt/lists/*

# Set up LLVM alternatives to use version 18 as default
RUN update-alternatives --install /usr/bin/llvm-config llvm-config /usr/bin/llvm-config-18 100 && \
    update-alternatives --install /usr/bin/clang clang /usr/bin/clang-18 100 && \
    update-alternatives --install /usr/bin/clang++ clang++ /usr/bin/clang++-18 100 && \
    update-alternatives --install /usr/bin/lld lld /usr/bin/lld-18 100 && \
    update-alternatives --install /usr/bin/ld.lld ld.lld /usr/bin/ld.lld-18 100

# Install Go 1.24
ENV GO_VERSION=1.24.4
RUN curl -fsSL https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz | tar -C /usr/local -xz

# Set Go environment variables
ENV PATH="/usr/local/go/bin:${PATH}"
ENV GOPATH="/root/go"
ENV PATH="${GOPATH}/bin:${PATH}"

# Install Zig (latest stable)
ENV ZIG_VERSION=0.13.0
RUN curl -fsSL https://ziglang.org/download/${ZIG_VERSION}/zig-linux-x86_64-${ZIG_VERSION}.tar.xz | tar -C /opt -xJ && \
    ln -s /opt/zig-linux-x86_64-${ZIG_VERSION}/zig /usr/local/bin/zig

# Install Node.js (for docs development - optional but useful)
RUN curl -fsSL https://deb.nodesource.com/setup_20.x | bash - && \
    apt-get install -y nodejs && \
    rm -rf /var/lib/apt/lists/*

# Set working directory
WORKDIR /zeus

# Set up shell prompt to indicate we're in the container
RUN echo 'export PS1="\[\033[1;35m\]⚡zeus-dev\[\033[0m\]:\[\033[1;34m\]\w\[\033[0m\]\$ "' >> /root/.bashrc

# Add helpful aliases
RUN echo 'alias ll="ls -la"' >> /root/.bashrc && \
    echo 'alias gs="git status"' >> /root/.bashrc && \
    echo 'alias test-all="make test"' >> /root/.bashrc && \
    echo 'alias build-runtime="make build-runtime"' >> /root/.bashrc && \
    echo 'alias run-example="make run file=main"' >> /root/.bashrc

# Print version info on container start
RUN echo 'echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"' >> /root/.bashrc && \
    echo 'echo "⚡ Zeus Linux Development Environment"' >> /root/.bashrc && \
    echo 'echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"' >> /root/.bashrc && \
    echo 'echo "Go:   $(go version | cut -d" " -f3)"' >> /root/.bashrc && \
    echo 'echo "Zig:  $(zig version)"' >> /root/.bashrc && \
    echo 'echo "LLVM: $(llvm-config --version)"' >> /root/.bashrc && \
    echo 'echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"' >> /root/.bashrc && \
    echo 'echo ""' >> /root/.bashrc && \
    echo 'echo "Quick commands:"' >> /root/.bashrc && \
    echo 'echo "  make test          - Run all tests"' >> /root/.bashrc && \
    echo 'echo "  make test-e2e      - Run e2e tests"' >> /root/.bashrc && \
    echo 'echo "  make build-runtime - Build Zig runtime"' >> /root/.bashrc && \
    echo 'echo "  make run file=main - Run playground/main.zs"' >> /root/.bashrc && \
    echo 'echo ""' >> /root/.bashrc

# Default command
CMD ["/bin/bash"]

