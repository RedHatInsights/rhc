#!/bin/bash
set -ux

# get to project root
cd ../../../

# Check for bootc/image-mode deployments which should not run dnf
if ! command -v bootc >/dev/null || bootc status | grep -q 'type: null'; then
  dnf --setopt install_weak_deps=False install -y \
    podman git-core python3-pip python3-pytest logrotate insights-client
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

# Copr builds on Rhel 9 has default configuration to connect to local broker
# Updating config.toml to connect to prod server during tests
if [ -f /etc/yum.repos.d/copr_build-RedHatInsights-rhc-* ]; then
  mv /etc/rhc/config.toml /etc/rhc/config.toml.bak
  cp systemtest/test_config.toml /etc/rhc/config.toml
fi

pytest --junit-xml=./junit.xml -v integration-tests
retval=$?

if [ -d "$TMT_PLAN_DATA" ]; then
  cp ./junit.xml "$TMT_PLAN_DATA/junit.xml"
  cp -r ./artifacts "$TMT_PLAN_DATA/"
fi

exit $retval
