#!/bin/bash
# Build llama.cpp library for cross-platform embedding support
# Output: dist/<os>-<arch>/libllama_go.<ext>
#
# Usage:
#   ./scripts/build-llama.sh          # Build for current platform
#   ./scripts/build-llama.sh all      # Build for all platforms
#
# Requirements:
#   - CMake >= 3.16
#   - GCC/Clang with C++17 support
#   - git

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
DIST_DIR="$PROJECT_ROOT/dist"
LLAMA_REPO="https://github.com/ggerganov/llama.cpp.git"
LLAMA_COMMIT="b3525e2"  # Pin to known-good commit for reproducibility

# Parse arguments
BUILD_ALL=false
if [[ "${1:-}" == "all" ]]; then
    BUILD_ALL=true
fi

# Detect current platform
detect_platform() {
    local os arch
    os=$(uname -s | tr '[:upper:]' '[:lower:]')
    arch=$(uname -m)

    case "$arch" in
        x86_64)  arch="amd64" ;;
        aarch64|arm64) arch="arm64" ;;
        *) echo "Unsupported architecture: $arch" >&2; exit 1 ;;
    esac

    echo "$os-$arch"
}

# Get library extension for platform
get_ext() {
    local os=$1
    case "$os" in
        darwin) echo "dylib" ;;
        linux)  echo "so" ;;
        windows) echo "dll" ;;
        *) echo "so" ;;
    esac
}

# Build for a specific platform
build_for_platform() {
    local os=$1
    local arch=$2
    local build_dir="/tmp/llama.cpp-build-${os}-${arch}"
    local output_dir="$DIST_DIR/${os}-${arch}"
    local ext=$(get_ext "$os")
    local lib_name="libllama_go.${ext}"

    echo "=========================================="
    echo "Building llama.cpp for $os/$arch"
    echo "=========================================="

    # Create output directory
    mkdir -p "$output_dir"

    # Clone or update llama.cpp
    if [[ -d "/tmp/llama.cpp" ]]; then
        echo "Updating existing llama.cpp..."
        cd /tmp/llama.cpp
        git fetch origin
        git checkout "$LLAMA_COMMIT"
    else
        echo "Cloning llama.cpp..."
        git clone --depth 1 --branch master "$LLAMA_REPO" /tmp/llama.cpp
        cd /tmp/llama.cpp
        git checkout "$LLAMA_COMMIT"
    fi

    # Clean build directory
    rm -rf "$build_dir"
    mkdir -p "$build_dir"
    cd "$build_dir"

    # Configure with CMake
    echo "Configuring CMake..."
    cmake -DCMAKE_BUILD_TYPE=Release \
          -DLLAMA_BUILD_SERVER=OFF \
          -DLLAMA_BUILD_EXAMPLES=OFF \
          -DLLAMA_BENCHMARK=OFF \
          -DLLAMA_CURL=OFF \
          -DLLAMA_SERVER=OFF \
          -DBUILD_SHARED_LIBS=ON \
          -DCMAKE_POSITION_INDEPENDENT_CODE=ON \
          /tmp/llama.cpp

    # Build
    echo "Building library..."
    case "$os" in
        darwin)
            cmake --build . --config Release -j$(sysctl -n hw.ncpu)
            ;;
        linux)
            cmake --build . --config Release -j$(nproc)
            ;;
        windows)
            cmake --build . --config Release --parallel
            ;;
    esac

    # Find and copy output
    echo "Copying output..."
    local lib_path
    case "$os" in
        darwin)
            lib_path=$(find . -name "libllama.${ext}" -type f | head -1)
            ;;
        linux)
            lib_path=$(find . -name "libllama.${ext}*" -type f | grep -v "\.a$" | head -1)
            ;;
        windows)
            lib_path=$(find . -name "llama.${ext}" -type f | head -1)
            ;;
    esac

    if [[ -z "$lib_path" ]]; then
        echo "Error: Library not found" >&2
        exit 1
    fi

    cp "$lib_path" "$output_dir/$lib_name"
    echo "Built: $output_dir/$lib_name"

    # Cleanup
    rm -rf "$build_dir"
}

# Build for current platform
echo "Building llama.cpp for current platform..."
platform=$(detect_platform)
os="${platform%-*}"
arch="${platform#*-}"
build_for_platform "$os" "$arch"

echo ""
echo "=========================================="
echo "Build complete!"
echo "Output: $DIST_DIR/$platform/"
ls -la "$DIST_DIR/$platform/"
echo "=========================================="
