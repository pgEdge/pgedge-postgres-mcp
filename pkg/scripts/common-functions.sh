#!/bin/bash

#--------------------------------------------------------------------------
#
# pgEdge MCP Packaging Common Functions
#
# Copyright (c) 2025 - 2026, pgEdge, Inc.
# This software is released under The PostgreSQL License
#
#--------------------------------------------------------------------------

install_syft(){

  echo "Installing syft ..."
  curl -sSfL https://raw.githubusercontent.com/anchore/syft/main/install.sh | sudo sh -s -- -b /usr/local/bin
}

setup_dnf_build_env(){

  echo "Installing required packages..."
  dnf groupinstall "Development Tools" -y
  dnf install -y rpm-build rpmdevtools yum-utils tar wget git gnupg2 sudo

  echo "📦 Enabling additional repositories..."
  dnf install -y epel-release
  if [ "$RHEL" = "8" ]; then
    dnf config-manager --set-enabled powertools
  else
    dnf config-manager --set-enabled crb
  fi

  echo "Configuring pgEdge repository..."
  configure_pgedge_dnf_repo $REPO_TYPE

  echo "Setting up RPM build environment..."
  rpmdev-setuptree

  install_syft
}

setup_apt_build_env(){

  echo "Installing build tools and dependencies..."
  sudo ln -fs /usr/share/zoneinfo/UTC /etc/localtime

  sudo apt-get update
  sudo apt-get install -y devscripts build-essential pkg-config fakeroot git curl \
    ca-certificates debhelper dpkg-dev gnupg2 wget sudo lsb-release

  echo "Configuring pgEdge repository..."
  configure_pgedge_apt_repo $REPO_TYPE

  install_syft
}

rename_ddeb_packages(){

  BUILD_DIR=$1
  pushd $BUILD_DIR
  for file in $(ls | grep ddeb); do
    mv "$file" "${file%.ddeb}.deb";
  done
  popd
}

configure_pgedge_dnf_repo() {
  local REPO_TYPE="${1:-daily}"       # "daily" or "staging"

  sudo dnf install -y https://dnf.pgedge.com/reporpm/pgedge-release-latest.noarch.rpm
  sudo sed -i "s|release|$REPO_TYPE|g" /etc/yum.repos.d/pgedge.repo

  echo "Repo configured at /etc/yum.repos.d/pgedge.repo"
}

configure_pgedge_apt_repo(){
  local REPO_TYPE="${1:-daily}"       # "daily" or "staging"
  local REPO_PATH="repodeb"

  curl -sSL https://apt.pgedge.com/${REPO_PATH}/pgedge-release_latest_all.deb -o /tmp/pgedge-release.deb && sudo dpkg -i /tmp/pgedge-release.deb && rm -f /tmp/pgedge-release.deb || true
  sed -i "s|release|$REPO_TYPE|g" /etc/apt/sources.list.d/pgedge.sources
  apt-get update

  echo "Repo configured at /etc/apt/sources.list.d/pgedge.sources"
}

detect_os_type(){
  if command -v dnf &>/dev/null || command -v yum &>/dev/null; then
    echo "Detected RPM-based system"
    source "${COMPONENT_DIR}/build-rpm.sh"
  elif command -v apt-get &>/dev/null; then
    echo "Detected Debian-based system"
    source "${COMPONENT_DIR}/build-deb.sh"
  else
    echo "Unsupported platform: No known package manager found" >&2
    exit 1
  fi
}

import_gpg_keys() {
  if ! command -v rpm &>/dev/null || ! command -v gpg &>/dev/null; then
    echo "Installing rpm or gpg"
    if command -v dnf &>/dev/null; then
      sudo dnf install -y rpm gnupg2
    elif command -v apt-get &>/dev/null; then
      sudo apt-get install -y rpm gnupg2
    fi
    if [ $? -ne 0 ]; then
      echo "Error: Failed to install rpm or gnupg2"
      return 1
    fi
  fi

  PRI_FILE="${SCRIPT_DIR}/public.key"
  PUB_FILE="${SCRIPT_DIR}/private.key"

  GPG_PUBLIC_KEY=$(cat $PRI_FILE)
  GPG_PRIVATE_KEY=$(cat $PUB_FILE)
  rm -f $PRI_FILE $PUB_FILE

  [ -z "$GPG_PUBLIC_KEY" ] && { echo "Error: GPG_PUBLIC_KEY is unset"; return 1; }
  [ -z "$GPG_PRIVATE_KEY" ] && { echo "Error: GPG_PRIVATE_KEY is unset"; return 1; }

  PUBLIC_KEY_FILE=$(mktemp)
  echo "$GPG_PUBLIC_KEY" > "$PUBLIC_KEY_FILE"

  gpg --import "$PUBLIC_KEY_FILE" || {
    echo "Error: Failed to import public key"
    rm -f "$PUBLIC_KEY_FILE"
    return 1
  }

  rpm --import "$PUBLIC_KEY_FILE" || {
    echo "Error: Failed to import public key to RPM"
    rm -f "$PUBLIC_KEY_FILE"
    return 1
  }

  PRIVATE_KEY_FILE=$(mktemp)
  echo "$GPG_PRIVATE_KEY" > "$PRIVATE_KEY_FILE"
  gpg --import "$PRIVATE_KEY_FILE" || {
    echo "Error: Failed to import private key"
    rm -f "$PRIVATE_KEY_FILE"
    rm -f "$PUBLIC_KEY_FILE"
    return 1
  }
  rm -f "$PRIVATE_KEY_FILE"
  rm -f "$PUBLIC_KEY_FILE"
  return 0
}

sign_rpms() {
  # Check if at least one file is provided
  if [ $# -eq 0 ]; then
    echo "Error: No files provided to sign."
    return 1
  fi

  # Check if rpmsign and gpg are installed, install if not
  if ! command -v rpmsign &>/dev/null; then
    echo "rpmsign not found. Installing rpm-sign"
    if command -v sudo &>/dev/null; then
      sudo dnf install -y rpm-sign
    else
      dnf install -y rpm-sign
    fi
    if [ $? -ne 0 ]; then
      echo "Error: Failed to install rpm-sign"
      return 1
    fi
  fi

  # Get the key ID of the imported private key
  KEY_ID=$(gpg --list-secret-keys --with-colons | awk -F: '/^sec/{print $5}' | head -n 1)
  if [ -z "$KEY_ID" ]; then
    echo "Error: No private key found after import."
    rm -f "$PRIVATE_KEY_FILE"
    rm -rf "$GNUPGHOME"
    return 1
  fi

  echo "=======================Signing RPMs======================="
  # Sign each RPM file
  for file in "$@"; do
    # Ensure the file has /output/ prefix if relative
    if [[ ! "$file" = /* ]]; then
      file="/output/$file"
    fi

    if [ ! -f "$file" ]; then
      echo "Error: File '$file' does not exist."
      continue
    fi

    # Check if the file is an RPM
    if ! file "$file" | grep -q "RPM"; then
      echo "Error: File '$file' is not an RPM file."
      continue
    fi

    # Sign the RPM using rpmsign, using passphrase if provided
    rpmsign --define "_gpg_name $KEY_ID" --addsign "$file" >/dev/null 2>&1

    if [ $? -eq 0 ]; then
      echo "Successfully signed '$file'."
    else
      echo "Error: Failed to sign '$file'."
    fi
  done
  echo "=======================Signing Completes=================="

  # Clean up
  rm -f "$PRIVATE_KEY_FILE"
}

validate_signatures() {

  # Check if files are provided
  if [ $# -eq 0 ]; then
    echo "Error: No files provided to validate."
    return 1
  fi

  # Install dependencies
  if ! command -v rpm &>/dev/null &>/dev/null; then
    echo "Installing rpm"
    if command -v sudo &>/dev/null; then
      sudo dnf install -y rpm
    else
      dnf install -y rpm
    fi
    if [ $? -ne 0 ]; then
      echo "Error: Failed to install rpm"
      return 1
    fi
  fi

  # Validate each RPM
  local all_valid=0
  echo "=======================Starting validation======================="
  for file in "$@"; do
    if [[ ! "$file" = /* ]]; then
      file="/output/$file"
    fi

    if [ ! -f "$file" ]; then
      echo "Error: File '$file' does not exist."
      all_valid=1
      continue
    fi

    if ! file "$file" | grep -q "RPM"; then
      echo "Error: File '$file' is not an RPM file."
      all_valid=1
      continue
    fi

    CHECKSIG_OUTPUT=$(rpm --checksig "$file" 2>&1)
    echo "$CHECKSIG_OUTPUT"
    if echo "$CHECKSIG_OUTPUT" | grep -q "digests signatures OK"; then
      echo "Signature for '$file' is valid."
    else
      echo "Error: Signature for '$file' is invalid or missing."
      all_valid=1
    fi
  done
  echo "=======================Validation completes======================"
  # Clean up
  rm -f "$PUBLIC_KEY_FILE"

  return $all_valid
}
