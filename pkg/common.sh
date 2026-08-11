#!/usr/bin/env bash

#--------------------------------------------------------------------------
#
# pgEdge MCP Packaging Environment
#
# Copyright (c) 2025 - 2026, pgEdge, Inc.
# This software is released under The PostgreSQL License
#
#--------------------------------------------------------------------------

# common.sh - packaging environment for pgedge-postgres-mcp.
#
# Sourced by pkg/scripts/build.sh (via the common/build.sh bridge wrapper)
# before common-functions.sh. Sets the version variables consumed by
# build-rpm.sh / build-deb.sh and the RPM spec / debian rules.

export PGEDGE_NLA_REPO="https://github.com/pgEdge/pgedge-postgres-mcp.git"
# Full tag (e.g. v1.0.0-beta3) — GoReleaser archives are named after this.
export PGEDGE_NLA_BRANCH="${COMPONENT_BRANCH:-v1.0.0}"
# Upstream version, suffix-stripped (e.g. 1.0.0) — used in spec/SOURCES names.
export PGEDGE_NLA_VERSION=${COMPONENT_VERSION:-1.0.0}
export PGEDGE_NLA_BUILDNUM=${COMPONENT_BUILDNUM:-1}

export REPO_TYPE="${REPO_TYPE:-daily}"

# DEB only: move a pre-release pretag (e.g. BUILDNUM='beta3_1') into the
# upstream VERSION with a leading '~' (1.0.0~beta3, BUILDNUM=1) so '~'
# sorts pre-releases BELOW stable in dpkg/reprepro.
if command -v apt-get &>/dev/null; then
    if [[ "$PGEDGE_NLA_BUILDNUM" == *_* ]]; then
        PGEDGE_NLA_PRETAG="${PGEDGE_NLA_BUILDNUM%%_*}"
        export PGEDGE_NLA_VERSION="${PGEDGE_NLA_VERSION}~${PGEDGE_NLA_PRETAG}"
        PGEDGE_NLA_BUILDNUM="${PGEDGE_NLA_BUILDNUM#*_}"
    fi
fi
