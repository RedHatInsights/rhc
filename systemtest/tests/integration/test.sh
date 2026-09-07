#!/bin/bash
set -ux

# Resolve repo root from this script so it works both when tmt runs it from
# systemtest/tests/integration/ and when invoked from the repository root.
SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
cd "${SCRIPT_DIR}/../../.." || exit 1

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

# Compose/CTC/RC sets PIN_TESTS_TO_RPM=1. Restore integration-tests/ from the
# git tag matching the installed rhc Version (v0.3.12, or unprefixed 0.2.x).
# Only the tests tree is restored so this test.sh (which is not on old tags)
# keeps running. PR and gating leave the flag unset.
pin_tests_to_installed_rhc() {
  if [[ "${PIN_TESTS_TO_RPM:-}" != "1" ]]; then
    echo "PIN_TESTS_TO_RPM is not set; using integration-tests from the current checkout"
    return 0
  fi
  if [[ -n "${TEST_RPMS:-}" ]]; then
    echo "TEST_RPMS is set (gating); not pinning tests to a release tag"
    return 0
  fi
  if ! command -v git >/dev/null; then
    echo "ERROR: git is required to pin tests to the installed rhc tag" >&2
    return 1
  fi
  if ! git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
    echo "ERROR: not a git checkout; cannot pin tests to a release tag" >&2
    return 1
  fi
  if ! rpm -q rhc >/dev/null 2>&1; then
    echo "ERROR: rhc is not installed; cannot pin tests" >&2
    return 1
  fi

  local ver tag
  ver=$(rpm -q --qf '%{VERSION}' rhc)
  if [[ -z "${ver}" ]]; then
    echo "ERROR: could not read rhc Version from RPM" >&2
    return 1
  fi

  echo "Installed rhc Version=${ver}; resolving matching git tag"
  git fetch --force origin "refs/tags/v${ver}:refs/tags/v${ver}" || true
  if git rev-parse -q --verify "refs/tags/v${ver}" >/dev/null; then
    tag="v${ver}"
  else
    git fetch --force origin "refs/tags/${ver}:refs/tags/${ver}" || true
    if git rev-parse -q --verify "refs/tags/${ver}" >/dev/null; then
      tag="${ver}"
    else
      echo "ERROR: no git tag v${ver} or ${ver} for installed rhc-${ver}" >&2
      return 1
    fi
  fi

  echo "Replacing integration-tests/ with the tree from tag ${tag}"
  # git checkout TAG -- dir overlays files but leaves newer tests in place.
  # Remove the directory first so pytest only sees the tagged suite.
  rm -rf integration-tests
  git checkout "${tag}" -- integration-tests
  git log -1 --oneline "${tag}"
  git status --short -- integration-tests
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

pin_tests_to_installed_rhc || exit 1

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

if [ -d "${TMT_PLAN_DATA:-}" ]; then
  cp ./junit.xml "$TMT_PLAN_DATA/junit.xml"
  cp -r ./artifacts "$TMT_PLAN_DATA/"
fi

exit $retval
