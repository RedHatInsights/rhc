#!/bin/bash
set -ux

# get to project root
cd ../../../

# Check for bootc/image-mode deployments which should not run dnf
if ! command -v bootc >/dev/null || bootc status | grep -q 'type: null'; then
  # Check for GitHub pull request ID and install build if needed.
  # This is for the downstream PR jobs.
  [ -z "${ghprbPullId+x}" ] || ./systemtest/copr-setup.sh

  dnf --setopt install_weak_deps=False install -y \
    podman git-core python3-pip python3-pytest logrotate
fi

if [[ -n "${TEST_RPMS+x}" ]]; then
	echo "Installing RPMs: ${TEST_RPMS}"
	dnf -y install --allowerasing ${TEST_RPMS}
fi

python3 -m venv venv
# shellcheck disable=SC1091
. venv/bin/activate

pip install -r integration-tests/requirements.txt

pytest --log-level debug  --junit-xml=./junit.xml -v integration-tests 
retval=$?

if [ -d "$TMT_PLAN_DATA" ]; then
  cp ./junit.xml "$TMT_PLAN_DATA/junit.xml"
  cp -r ./artifacts "$TMT_PLAN_DATA/"
fi

exit $retval
