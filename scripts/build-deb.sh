#!/bin/bash
set -e

# Build Debian package for gotunnel
# Usage: ./scripts/build-deb.sh [version]

VERSION=${1:-"0.1.0-beta"}
ARCH=${2:-"amd64"}
BUILD_DIR="build/debian"
PACKAGE_NAME="gotunnel"

echo "Building Debian package v${VERSION} for ${ARCH}..."

# Clean and create build directory
rm -rf "${BUILD_DIR}"
mkdir -p "${BUILD_DIR}"

# Create package directory structure
PKG_DIR="${BUILD_DIR}/${PACKAGE_NAME}_${VERSION}_${ARCH}"
mkdir -p "${PKG_DIR}/DEBIAN"
mkdir -p "${PKG_DIR}/usr/bin"
mkdir -p "${PKG_DIR}/usr/share/gotunnel"
mkdir -p "${PKG_DIR}/etc/gotunnel"

# Copy control files
cp packaging/debian/control "${PKG_DIR}/DEBIAN/"
cp packaging/debian/postinst "${PKG_DIR}/DEBIAN/"
cp packaging/debian/prerm "${PKG_DIR}/DEBIAN/"

# Update version in control file
sed -i "s/Version: .*/Version: ${VERSION}/" "${PKG_DIR}/DEBIAN/control"
sed -i "s/Architecture: .*/Architecture: ${ARCH}/" "${PKG_DIR}/DEBIAN/control"

# Make scripts executable
chmod 755 "${PKG_DIR}/DEBIAN/postinst"
chmod 755 "${PKG_DIR}/DEBIAN/prerm"

# Build the binary if it doesn't exist
if [ ! -f "gotunnel" ]; then
    echo "Building gotunnel binary..."
    CGO_ENABLED=0 GOOS=linux GOARCH=${ARCH} go build \
        -ldflags="-s -w -X main.version=${VERSION} -X main.commit=$(git rev-parse HEAD) -X main.date=$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
        -o gotunnel ./cmd/gotunnel
fi

# Copy binary and configuration
cp gotunnel "${PKG_DIR}/usr/bin/"
cp configs/gotunnel.example.yaml "${PKG_DIR}/usr/share/gotunnel/"
cp configs/gotunnel.example.yaml "${PKG_DIR}/etc/gotunnel/config.yaml"

# Set permissions
chmod 755 "${PKG_DIR}/usr/bin/gotunnel"
chmod 644 "${PKG_DIR}/usr/share/gotunnel/gotunnel.example.yaml"
chmod 644 "${PKG_DIR}/etc/gotunnel/config.yaml"

# Build the package
echo "Building package..."
dpkg-deb --build "${PKG_DIR}"

# Move to final location
DEB_FILE="${PACKAGE_NAME}_${VERSION}_${ARCH}.deb"
mv "${PKG_DIR}.deb" "${BUILD_DIR}/${DEB_FILE}"

echo "✅ Debian package built successfully: ${BUILD_DIR}/${DEB_FILE}"

# Calculate checksums
cd "${BUILD_DIR}"
sha256sum "${DEB_FILE}" > "${DEB_FILE}.sha256"
echo "📝 Checksum: $(cat ${DEB_FILE}.sha256)"

# Test the package (if dpkg is available)
if command -v dpkg &> /dev/null; then
    echo "🔍 Testing package..."
    dpkg --info "${DEB_FILE}"
    dpkg --contents "${DEB_FILE}"
fi

echo "🎉 Package ready for distribution!"