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

  if [[ "${ID}" = "centos" ]]; then
    # Do not try to install insights-client on CentOS Stream 9, because it cannot be installed there.
    # Try to only mock insights-client to be able to test behavior of rhc on CentOS Stream 9.
    mock_insights_client
  else
    # Try to install insights-client on RHEL as a weak dependency
    dnf --setopt install_weak_deps=True install -y insights-client
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
# Re-assert the dynaconf security floor (CVE-2026-33154, SSTI in the
# @jinja resolver, fixed in 3.2.13). The authoritative constraint lives in
# integration-tests/requirements.txt, and pytest-client-tools declares the
# same floor in its own metadata, so even a requirements file restored
# from a pre-pin release tag (PIN_TESTS_TO_RPM=1) already resolves it;
# this line is a belt-and-braces re-assertion so the floor cannot regress
# if those upstream constraints are ever dropped. dynaconf 3.2.13
# requires python >= 3.8; EL8 guests (python 3.6, still gated through the
# release branches) stay on the 3.1.x line instead.
pip install 'dynaconf>=3.2.13,<4; python_version >= "3.8"' 'dynaconf>=3.1.5,<3.2; python_version < "3.8"'

# If SETTINGS_URL is set (most likely in .testing-farm.yaml), download the
# sanitized settings file (non-secret values only) from the provided URL.
# Back up any existing settings.toml before downloading. Secrets are expected
# as PYTEST_CLIENT_TOOLS_* environment variables (see the CCT credentials
# how-to on the QE wiki).
if [[ -v SETTINGS_URL ]]; then
  [ -f ./settings.toml ] && mv ./settings.toml ./settings.toml.bak
  if ! curl -f "$SETTINGS_URL" -o ./settings.toml; then
    echo "ERROR: Failed to download settings from: $SETTINGS_URL" >&2
    exit 1
  fi
  # Strict mode: an @vault: sentinel that cannot be resolved fails
  # loudly at config load instead of degrading into a missing key and a
  # skipped run. Runs without SETTINGS_URL (containerized, manual) are
  # unaffected.
  export PYTEST_CLIENT_TOOLS_VAULT_REQUIRED=1
fi

# The sanitized settings file provides candlepin.username without the
# password, and pytest-client-tools requires a password whenever a
# username is set (activation keys alone do not satisfy that check). For
# the containerized candlepin (ENV_FOR_DYNACONF=testing) fall back to the
# container's well-known credentials; credentials for real environments
# must be supplied through PYTEST_CLIENT_TOOLS_* environment variables.
if [[ "${ENV_FOR_DYNACONF:-}" = "testing" ]]; then
  # Suppress tracing: Testing Farm masks only the values it injected
  # itself, so an operator-supplied real password must not reach the log.
  set +x
  export PYTEST_CLIENT_TOOLS_CANDLEPIN__USERNAME="${PYTEST_CLIENT_TOOLS_CANDLEPIN__USERNAME:-admin}"
  export PYTEST_CLIENT_TOOLS_CANDLEPIN__PASSWORD="${PYTEST_CLIENT_TOOLS_CANDLEPIN__PASSWORD:-admin}"
  set -x
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
