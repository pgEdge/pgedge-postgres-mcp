#!/usr/bin/env bash

#--------------------------------------------------------------------------
#
# pgEdge MCP Debian Packaging Script
#
# Copyright (c) 2025 - 2026, pgEdge, Inc.
# This software is released under The PostgreSQL License
#
#--------------------------------------------------------------------------

set -euo pipefail

# Environment variables
BUILD_DIR="/tmp/pg_deb_build"
SRC_DIR="${BUILD_DIR}/src"

CWD="$(pwd)"

export DEBIAN_FRONTEND=noninteractive
ARCH=$(uname -m)
if [ "$ARCH" = "aarch64" ]; then
  ARCH="arm64"
fi

# Full tag version incl any -beta/-rc suffix (GoReleaser archive names use it).
TAG_VERSION="${PGEDGE_NLA_BRANCH#v}"
ARTIFACT_DIR="${ARTIFACT_DIR:-${CWD}/release-artifacts}"
RELEASE_URL="https://github.com/pgEdge/pgedge-postgres-mcp/releases/download/${PGEDGE_NLA_BRANCH}"

# stage_tarball <canonical-local-name> <release-asset-name> <dest-path>
# Prefer a workflow-staged tarball under release-artifacts/ (so simulate_tag
# branch tests work and package cells don't depend on the GH release job);
# otherwise download the published release asset.
stage_tarball() {
  local local_name="$1" asset="$2" dest="$3"
  if [ -f "${ARTIFACT_DIR}/${local_name}" ]; then
    echo "Staging ${local_name} from ${ARTIFACT_DIR}"
    cp "${ARTIFACT_DIR}/${local_name}" "${dest}"
  else
    echo "Downloading ${asset} from release ${PGEDGE_NLA_BRANCH}"
    wget -q "${RELEASE_URL}/${asset}" -O "${dest}"
  fi
}

prepare() {

  setup_apt_build_env
  apt-get install -y jq
  # This function is for debugging purpose if you have your own keys. GH workflow does not need it.
  #import_gpg_keys

  echo "Resetting build workspace at ${SRC_DIR}..."
  rm -rf "$SRC_DIR"
  mkdir -p "$SRC_DIR"/{server,cli,web}

  echo "Staging GoReleaser tarballs..."
  stage_tarball "server.tar.gz" \
    "pgedge-postgres-mcp-server_${TAG_VERSION}_linux_${ARCH}.tar.gz" "${BUILD_DIR}/server.tar.gz"
  stage_tarball "cli.tar.gz" \
    "pgedge-postgres-mcp-cli_${TAG_VERSION}_linux_${ARCH}.tar.gz" "${BUILD_DIR}/cli.tar.gz"
  stage_tarball "web.tar.gz" \
    "pgedge-nla-web_${TAG_VERSION}_noarch.tar.gz" "${BUILD_DIR}/web.tar.gz"

  echo "Extracting tarballs..."
  tar -xzf "${BUILD_DIR}/server.tar.gz" -C "$SRC_DIR/server"
  tar -xzf "${BUILD_DIR}/cli.tar.gz"    -C "$SRC_DIR/cli"
  tar -xzf "${BUILD_DIR}/web.tar.gz"    -C "$SRC_DIR/web"

  echo "Copying Debian packaging files..."
  cp -r "${CWD}/${COMPONENT_NAME}/deb/debian" "$SRC_DIR/"

  # systemd unit + configs referenced by debian/rules
  cp "${CWD}/${COMPONENT_NAME}/common/pgedge-postgres-mcp.service" "$SRC_DIR/debian/"
  cp "${CWD}/${COMPONENT_NAME}/common/postgres-mcp.yaml" "$SRC_DIR/debian/"
  cp "${CWD}/${COMPONENT_NAME}/common/postgres-mcp.env" "$SRC_DIR/debian/"
  cp "${CWD}/${COMPONENT_NAME}/common/nla-cli.yaml" "$SRC_DIR/debian/"
  cp "${CWD}/${COMPONENT_NAME}/common/pgedge-nla-web.nginx" "$SRC_DIR/debian/"

  echo "Installing build dependencies..."
  cd "$SRC_DIR"
  sudo apt-get update
  sudo apt-get build-dep -y .

  echo "Preparation complete. Source directory: $SRC_DIR"
}

build() {

  cd "$SRC_DIR"

  echo "Building Debian packages..."
  DISTRO=$(lsb_release -cs)

  # Generate changelog
  cat > debian/changelog <<EOF
pgedge-nla (${PGEDGE_NLA_VERSION}-${PGEDGE_NLA_BUILDNUM}.${DISTRO}) ${DISTRO}; urgency=medium

  * Update packages of MCP server, CLI and web

 -- pgEdge Build Team <support@pgedge.com>  $(date -R)
EOF

  # Build packages
  dpkg-buildpackage -us -uc -b
}

post_build() {
  echo "Copying .deb packages to output..."
  sudo mkdir -p "/output"
  # Rename .ddeb files to .deb files
  rename_ddeb_packages $BUILD_DIR
  sudo cp "$BUILD_DIR"/*.deb "/output" || echo "No .deb packages found."
}
