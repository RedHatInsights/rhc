#!/bin/bash
set -ux

# get to project root
cd ../../../

# Read information about release from standard release file
if [[ -f "/etc/os-release" ]]; then
  source /etc/os-release
fi

function mock_insights_client() {
  # Create configuration directory, when it does not exists
  if [[ ! -d "/etc/insights-client/" ]]; then
    mkdir -p /etc/insights-client/
  fi
  # Create empty configuration file
  if [[ ! -f "/etc/insights-client/insights-client.conf" ]]; then
    touch /etc/insights-client/insights-client.conf
  fi

  # Create a mock of insights-client.
  #
  # To mimic behavior of original client we return 0, when
  # the system is registered (the consumer cert is installed)
  # Otherwise, it returns non-zero value. The original
  # insights-client also creates hidden files in
  # the /etc/insights-client directory. When the insights-client
  # is registered, then there is .registered file, and when
  # the insights-client is unregistered, then there is
  # .unregistered file. This behavior should be enough
  # to make rhc happy.
  if [[ ! -f "/bin/insights-client" ]]; then
    cat >/bin/insights-client << 'EOF'
#!/bin/bash

if [[ -f /etc/pki/consumer/cert.pem ]]
then
	touch /etc/insights-client/.registered
	rm -f /etc/insights-client/.unregistered
	exit 0
else
	touch /etc/insights-client/.unregistered
	rm -f /etc/insights-client/.registered
	exit 1
fi
EOF
    chmod a+x /bin/insights-client
  fi
}

is_bootc() {
  command -v bootc > /dev/null && \
  ! bootc status --format=humanreadable | grep -q 'System is not deployed via bootc'
}

if is_bootc; then
  echo "System is deployed via bootc, skipping dnf install"
else
  # In most cases these should already be installed by tmt, see systemtest/plans/main.fmf
  # This is for running this script without tmt.
  dnf --setopt install_weak_deps=False install -y \
    podman git-core python3-pip python3-pytest logrotate

  if [[ "${ID}" = "fedora" ]]; then
    # Do not try to install insights-client on Fedora, because it cannot be installed there.
    # Try to only mock insights-client to be able to test behavior of rhc on Fedora.
    mock_insights_client
  else
    # Try to install insights-client on other Linux distributions
    dnf --setopt install_weak_deps=False install -y insights-client
  fi

  # TEST_RPMS is set in jenkins jobs after parsing CI Messages in gating Jobs.
  # If TEST_RPMS is set then install the RPM builds for gating.
  if [[ -v TEST_RPMS ]]; then
    echo "Installing RPMs: ${TEST_RPMS}"
    dnf -y install --allowerasing ${TEST_RPMS}
  fi
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
