#!/bin/bash

OUT_DIR="build"

if [ -d "$OUT_DIR" ]; then
    rm -rf "$OUT_DIR"
fi
mkdir -p "$OUT_DIR"

targets=(
    "windows/386"
    "windows/amd64"
    "windows/arm64"
    "linux/386"
    "linux/amd64"
    "linux/arm/6"
    "linux/arm"
    "linux/arm64"
)

for target in "${targets[@]}"; do
    IFS="/" read -r OS ARCH ARM <<< "$target"

    export GOARM=""
    export GOOS=$OS
    export GOARCH=$ARCH
    
    EXT=""
    if [ "$OS" == "windows" ]; then
        EXT=".exe"
    fi

    Y_NAME=$ARCH
    if [ "$ARCH" == "arm" ]; then
        if [ "$ARM" == "6" ]; then
            export GOARM=6
            Y_NAME="armv6l"
        else
            Y_NAME="armv7l"
        fi
    fi

    OUTPUT_NAME="${OUT_DIR}/lolcat-go-${OS}-${Y_NAME}${EXT}"

    echo "Building for ${OS}/${Y_NAME}..."
    go build -o "$OUTPUT_NAME" .
done