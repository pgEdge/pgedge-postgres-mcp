#!/usr/bin/env bash

#--------------------------------------------------------------------------
#
# pgEdge MCP Build Bridge Wrapper
#
# Copyright (c) 2025 - 2026, pgEdge, Inc.
# This software is released under The PostgreSQL License
#
#--------------------------------------------------------------------------

# Bridge wrapper: pgedge-builder-action hardcodes
# `cd /build && bash ./common/build.sh $COMPONENT_NAME`. Our packaging lives
# under pkg/, so delegate to the real entrypoint there. The shared
# pgedge-builder-action must not be modified — this wrapper adapts to it.
exec "$(dirname "$0")/../pkg/scripts/build.sh" "$@"
