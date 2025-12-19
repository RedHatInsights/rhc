#!/bin/bash
set -ux

source /etc/os-release

# dependencies to install
packages=(
  "git-core"
  "logrotate"
  "man-db"
  "podman"
  "python3-pip"
  "python3-pytest"
)

# add insights-client if not fedora
if [ "$ID" == "rhel" ] || [ "$ID" == "centos" ]; then
  packages+=(
    "insights-client"
  )
fi

# add rhc if not installed (eg. from packit) or TEST_RPMS is unset/blank
if ! command -v rhc > /dev/null || [ -z "${TEST_RPMS:-}" ]; then
  packages+=("rhc")
fi

get_image_name() {
  if command -v jq > /dev/null; then
    IMAGE=$(bootc status --format=json | jq -r '.status.booted.image.image.image')
  else
    IMAGE=$(bootc status --format=humanreadable | grep 'Booted image' | cut -d' ' -f 4)
  fi
  echo "$IMAGE"
}

is_bootc() {
  command -v bootc > /dev/null &&
    ! bootc status --format=humanreadable | grep -q 'System is not deployed via bootc'
}

if is_bootc; then
  echo "info: running in bootc/image-mode, preparing new image"
  IMAGE=$(get_image_name)
  echo "info: current image is $IMAGE"

  (podman pull $IMAGE || podman pull containers-storage:$IMAGE) || bootc image copy-to-storage --target $IMAGE
  podman build --build-arg IMAGE=$IMAGE -t localhost/rhc-test:latest -f Containerfile systemtest/

  echo "info: switching to new bootc image and rebooting"
  bootc switch --transport containers-storage localhost/rhc-test:latest
else
  echo "info: installing dependencies"
  dnf --setopt install_weak_deps=False install -y ${packages[@]}
  echo "info: dependencies installed successfully"

  # TEST_RPMS is set in jenkins jobs after parsing CI Messages in gating Jobs.
  # If TEST_RPMS is set then install the RPM builds for gating.
  if [[ -v TEST_RPMS ]]; then
    echo "Installing RPMs: ${TEST_RPMS}"
    dnf -y install --allowerasing ${TEST_RPMS}
  fi
fi
