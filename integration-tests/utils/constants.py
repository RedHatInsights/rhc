"""
Centralized constants for rhc integration tests.

All shared paths, service names, feature definitions, and other constants
used across test modules live here.

To add a new feature:
1. Add mapping entry in FEATURE_MAPPING (CLI name -> JSON name).
   The order must be the same as in the --help message.
2. If the feature has dependencies, add them to FEATURE_DEPENDENCIES.
3. Tests will automatically include the new feature in parameterized tests.
"""

# ---------------------------------------------------------------------------
# Filesystem paths
# ---------------------------------------------------------------------------
RHC_COLLECTOR = "/usr/libexec/rhc/rhc-collector"
TIMER_CACHE_DIR = "/var/cache/rhc/collectors"
COLLECTOR_CONFIG_DIR = "/usr/lib/rhc/collectors"
COLLECTOR_BIN_DIR = "/usr/libexec/rhc/collectors"
RHC_TMP_DIR = "/var/tmp/rhc"
LOG_FILE_PATH = "/var/log/rhc/rhc.log"
LOGROTATE_CONFIG_PATH = "/etc/logrotate.d/rhc"
CONNECT_FEATURES_PREFS_PATH = "/var/lib/rhc/rhc-connect-features-prefs.json"
REDHAT_REPO_FILE = "/etc/yum.repos.d/redhat.repo"
COMPLETION_SCRIPT = "/usr/share/bash-completion/completions/rhc"

# ---------------------------------------------------------------------------
# Varlink
# ---------------------------------------------------------------------------
VARLINK_SOCKET_ADDRESS = "unix:/run/rhc/com.redhat.rhc"
VARLINK_METHOD_COLLECTOR_INFO = "com.redhat.rhc.collector.Info"
VARLINK_METHOD_COLLECTOR_LIST = "com.redhat.rhc.collector.List"

# ---------------------------------------------------------------------------
# Systemd unit names
# ---------------------------------------------------------------------------
RHC_SERVER_SOCKET = "rhc-server.socket"
RHC_SERVER_SERVICE = "rhc-server.service"
YGGDRASIL_SERVICE_NAME = "yggdrasil"
YGGDRASIL_SERVICE_UNIT = "yggdrasil.service"
RHSM_SERVICE_UNIT = "rhsm.service"

# ---------------------------------------------------------------------------
# Shipped minimal collector
# ---------------------------------------------------------------------------
MINIMAL_COLLECTOR_ID = "com.redhat.minimal"
MINIMAL_COLLECTOR_NAME = "Minimal Host Inventory Collector"
MINIMAL_COLLECTOR_CONFIG_PATH = f"{COLLECTOR_CONFIG_DIR}/{MINIMAL_COLLECTOR_ID}.toml"
MINIMAL_TIMER_UNIT = f"rhc-collector-{MINIMAL_COLLECTOR_ID}.timer"
MINIMAL_SERVICE_UNIT = f"rhc-collector-{MINIMAL_COLLECTOR_ID}.service"

# ---------------------------------------------------------------------------
# Feature definitions
# ---------------------------------------------------------------------------
FEATURE_MAPPING = {
    "content": "content",
    "analytics": "analytics",
    "remote-management": "remote_management",
}
ALL_FEATURES_CLI = list(FEATURE_MAPPING.keys())
ALL_FEATURES_JSON = list(FEATURE_MAPPING.values())
FEATURE_DEPENDENCIES = {
    "remote-management": ["content", "analytics"],
}
CONFIGURE_FEATURES_STATUS_JSON_KEYS = frozenset({"connected", "features"})
CONFIGURE_FEATURES_JSON_FEATURE_KEYS = frozenset(FEATURE_MAPPING.values())

# ---------------------------------------------------------------------------
# OAuth
# ---------------------------------------------------------------------------
OAUTH_DEFAULT_SCOPE = "openid api.iam.service_accounts"

# ---------------------------------------------------------------------------
# CLI exit codes
# ---------------------------------------------------------------------------
EXIT_CODE_USAGE = 64
EXIT_CODE_DATA_FORMAT = 65

# ---------------------------------------------------------------------------
# Test data
# ---------------------------------------------------------------------------
INVALID_CREDENTIAL = "xpto123"
