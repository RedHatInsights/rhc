#!/bin/bash
set -x

is_bootc() {
  command -v bootc > /dev/null &&
    ! bootc status --format=humanreadable | grep -q 'System is not deployed via bootc'
}

REBOOT_MARKER="/var/tmp/rhc-test-deps-installed"

if is_bootc; then
  if [ ! -f "${REBOOT_MARKER}" ]; then
    echo "info: first boot after bootc switch, creating reboot marker"
    if touch "${REBOOT_MARKER}"; then
      echo "info: rebooting once to continue tests"
      tmt-reboot
    else
      echo "error: failed to create reboot marker ${REBOOT_MARKER}; refusing to reboot to avoid loop"
      exit 1
    fi
  else
    echo "info: reboot marker found, continuing without reboot"
  fi
else
  echo "info: not a bootc system, skipping post-reboot setup"
fi
