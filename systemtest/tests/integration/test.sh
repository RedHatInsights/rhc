#!/bin/bash
set -ux

# get to project root
cd ../../../

# Read information about release from standard release file
if [[ -f "/etc/os-release" ]]; then
  source /etc/os-release
fi

# Check for bootc/image-mode deployments which should not run dnf
if ! command -v bootc >/dev/null || bootc status | grep -q 'type: null'; then
  # Check for GitHub pull request ID and install build if needed.
  # This is for the downstream PR jobs.
  [ -z "${ghprbPullId+x}" ] || ./systemtest/copr-setup.sh

  if [[ "${ID}" = "fedora" ]]; then
    # Do not try to install insights-client on Fedora, because it cannot be installed there
    dnf --setopt install_weak_deps=False install -y \
      podman git-core python3-pip python3-pytest logrotate
  else
    dnf --setopt install_weak_deps=False install -y \
      podman git-core python3-pip python3-pytest logrotate insights-client
  fi
fi


# TEST_RPMS is set in jenkins jobs after parsing CI Messages in gating Jobs.
# If TEST_RPMS is set then install the RPM builds for gating.
if [[ -v TEST_RPMS ]]; then
	echo "Installing RPMs: ${TEST_RPMS}"
	dnf -y install --allowerasing ${TEST_RPMS}
fi

python3 -m venv venv
# shellcheck disable=SC1091
. venv/bin/activate
pip install --upgrade pip
pip install -r integration-tests/requirements.txt

# If SETTINGS_URL is set (most likely in .testing-farm.yaml), download the settings
# file from the provided URL. Back up any existing settings.toml before downloading.
if [[ -v SETTINGS_URL ]]; then
  [ -f ./settings.toml ] && mv ./settings.toml ./settings.toml.bak
  if ! curl -f "$SETTINGS_URL" -o ./settings.toml; then
    echo "ERROR: Failed to download settings from: $SETTINGS_URL" >&2
    exit 1
  fi
fi

pytest --junit-xml=./junit.xml -v integration-tests
retval=$?

if [ -d "$TMT_PLAN_DATA" ]; then
  cp ./junit.xml "$TMT_PLAN_DATA/junit.xml"
  cp -r ./artifacts "$TMT_PLAN_DATA/"
fi

exit $retval
