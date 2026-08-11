#!/bin/bash

#--------------------------------------------------------------------------
#
# pgEdge MCP RPM Packaging Script
#
# Copyright (c) 2025 - 2026, pgEdge, Inc.
# This software is released under The PostgreSQL License
#
#--------------------------------------------------------------------------

set -euo pipefail

RHEL="$(rpm --eval %rhel)"
ARCH=$(uname -m)
if [ "$ARCH" = "aarch64" ]; then
  ARCH="arm64"
fi

# Full tag version incl any -beta/-rc suffix (GoReleaser archive names use it);
# PGEDGE_NLA_VERSION is the suffix-stripped upstream version used in the spec
# and the SOURCES filenames.
TAG_VERSION="${PGEDGE_NLA_BRANCH#v}"
ARTIFACT_DIR="${ARTIFACT_DIR:-$(pwd)/release-artifacts}"
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
  setup_dnf_build_env
  dnf install -y jq

  echo "Copying packaging files..."
  cp ${COMPONENT_NAME}/rpm/pgedge-nla.spec ~/rpmbuild/SPECS/

  echo "Staging GoReleaser tarballs into SOURCES..."
  stage_tarball "server.tar.gz" \
    "pgedge-postgres-mcp-server_${TAG_VERSION}_linux_${ARCH}.tar.gz" \
    ~/rpmbuild/SOURCES/pgedge-postgres-mcp-server_${PGEDGE_NLA_VERSION}_linux_${ARCH}.tar.gz
  stage_tarball "cli.tar.gz" \
    "pgedge-postgres-mcp-cli_${TAG_VERSION}_linux_${ARCH}.tar.gz" \
    ~/rpmbuild/SOURCES/pgedge-postgres-mcp-cli_${PGEDGE_NLA_VERSION}_linux_${ARCH}.tar.gz
  stage_tarball "web.tar.gz" \
    "pgedge-nla-web_${TAG_VERSION}_noarch.tar.gz" \
    ~/rpmbuild/SOURCES/pgedge-nla-web_${PGEDGE_NLA_VERSION}_noarch.tar.gz

  echo "Copying service/config sources..."
  cp ${COMPONENT_NAME}/common/* ~/rpmbuild/SOURCES/

  # This function is for debugging purpose if you have your own keys. GH workflow does not need it.
  #import_gpg_keys

  echo "🔧 Installing RPM build dependencies..."
  dnf builddep -y \
    --define "pgedge_nla_version ${PGEDGE_NLA_VERSION}" \
    --define "pgedge_nla_buildnum ${PGEDGE_NLA_BUILDNUM}" \
    --define "arch ${ARCH}" \
    ~/rpmbuild/SPECS/pgedge-nla.spec
}

build() {
  echo "Building RPM and SRPM..."
  QA_RPATHS=$(( 0xffff )) rpmbuild -ba ~/rpmbuild/SPECS/pgedge-nla.spec \
    --define "pgedge_nla_version ${PGEDGE_NLA_VERSION}" \
    --define "pgedge_nla_buildnum ${PGEDGE_NLA_BUILDNUM}" \
    --define "arch ${ARCH}"
}

post_build() {
  echo "📤 Copying built RPMs to /output..."
  mkdir -p /output
  cp -v ~/rpmbuild/RPMS/*/*.rpm /output/ || echo "No binary RPMs found"
  cp -v ~/rpmbuild/SRPMS/*.src.rpm /output/ || echo "No SRPM found"

  sign_rpms /output/*.rpm
  validate_signatures /output/*.rpm
}
