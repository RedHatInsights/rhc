#!/bin/bash
set -ux

source /etc/os-release

packages=(
  "git-core"
  "logrotate"
  "man-db"
  "podman"
  "python3-pip"
  "python3-pytest"
  "python3-tomli"
  "rhc"
)

if [ "$ID" == "rhel" ] || [ "$ID" == "centos" ]; then
  packages+=(
    "insights-client"
  )
fi

if ! command -v rhc > /dev/null || [ -z "${TEST_RPMS:-}" ]; then
  packages+=("rhc")
fi

dnf --setopt install_weak_deps=False install -y "${packages[@]}"

# TEST_RPMS is set in jenkins jobs after parsing CI Messages in gating Jobs.
if [[ -v TEST_RPMS ]]; then
  echo "Installing RPMs: ${TEST_RPMS}"
  dnf -y install --allowerasing ${TEST_RPMS}
fi

# workaround - Install tomli for python 3.10 and below (RHEL8.x)
python3 -m pip install tomli
