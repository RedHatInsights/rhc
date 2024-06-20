#!/bin/bash
set -eu
set -x

# get to project root
cd ../../../

# Check for GitHub pull request ID and install build if needed.
# This is for the downstream PR jobs.
[ -z "${ghprbPullId+x}" ] || ./systemtest/copr-setup.sh

dnf --setopt install_weak_deps=False install -y \
  podman git-core python3-pip python3-pytest logrotate

python3 -m venv venv
# shellcheck disable=SC1091
. venv/bin/activate

pip install -r integration-tests/requirements.txt

pytest -v integration-tests
