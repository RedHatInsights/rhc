import os

VARLINK_SOCKET = "/run/rhc/com.redhat.rhc"

ENTITLEMENT_CERT_DIR = "/etc/pki/entitlement/"
ENTITLEMENT_BACKUP_DIR_PREFIX = "entitlement-backup-"
RELEASEVER_FILE = "/etc/dnf/vars/releasever"
RHSM_HOST_CONFIG_DIR = "/etc/rhsm-host"
PRODUCT_CERT_DIR = "/etc/pki/product/"
DEFAULT_PRODUCT_CERT_DIR = "/etc/pki/product-default/"
ENTITLEMENT_HOST_CERT_DIR = "/etc/pki/entitlement-host/"
RHC_SERVER_LOG_FILE = "/var/log/rhc/rhc-server.log"
DNF5_REPOS_OVERRIDE_DIR = "/etc/dnf/repos.override.d"
DNF5_REDHAT_REPOS_OVERRIDE_FILE = os.path.join(DNF5_REPOS_OVERRIDE_DIR, "98-redhat.repo")
