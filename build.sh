#!/bin/bash

# build_release.sh
# Usage: ./build_release.sh <version>
# Example: ./build_release.sh v1.2.3

set -e

if [ -z "$1" ]; then
  echo "Usage: $0 <version>"
  exit 1
fi

VERSION="$1"
APP_NAME="ssl-checker"
OUTPUT_DIR="release"
SIGN_KEY="signing_key.pem"   # Your private key for signing

rm -rf $OUTPUT_DIR
mkdir -p $OUTPUT_DIR

function build() {
  GOOS=$1
  GOARCH=$2
  EXT=$3
  TARGET="$APP_NAME-$VERSION-$GOOS-$GOARCH$EXT"
  echo "Building $TARGET"
  env GOOS=$GOOS GOARCH=$GOARCH go build -ldflags "-s -w -X 'ssl-checker/cmd.Version=$VERSION'" -o $TARGET .
  mkdir -p $OUTPUT_DIR/$TARGET
  mv $TARGET $OUTPUT_DIR/$TARGET/
  cp -r configs $OUTPUT_DIR/$TARGET/ 2>/dev/null || true
  cd $OUTPUT_DIR
  zip -r $TARGET.zip $TARGET
  # Sign the zip file
  if [ -f "../$SIGN_KEY" ]; then
    openssl dgst -sha256 -sign ../$SIGN_KEY -out $TARGET.zip.sig $TARGET.zip
  fi
  cd ..
  rm -rf $OUTPUT_DIR/$TARGET
}

# Build for linux, windows, macos (amd64)
build linux amd64 ""
build windows amd64 ".exe"
build darwin amd64 ""

# Build for macos arm64 (Apple Silicon)
build darwin arm64 ""

echo "All builds and packaging done."
echo "Output in $OUTPUT_DIR/"

# List output
ls -lh $OUTPUT_DIR/