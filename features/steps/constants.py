import os

VARLINK_SOCKET = "/run/rhc/com.redhat.rhc"

OVERRIDE_DIR = "/etc/dnf/repos.override.d"
OVERRIDE_FILE = os.path.join(OVERRIDE_DIR, "98-redhat.repo")
