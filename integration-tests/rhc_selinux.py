"""RHC-specific known SELinux AVC exceptions."""

_KNOWN_AVC_SKIPS = [
    {
        "fields": {
            "subj": "system_u:system_r:rhsmcertd_t:s0",
        }
    },  # https://redhat.atlassian.net/browse/CCT-2900
    {
        "fields": {
            "subj": "system_u:system_r:auditctl_t:s0",
            "permission": "read",
            "obj": "system_u:object_r:auditd_log_t:s0",
        }
    },  # RHEL 10 only: https://redhat.atlassian.net/browse/RHEL-246545
]


def avc_skips_for_test(_test_name=None):
    """Return RHC's known AVC exceptions for a test."""
    return list(_KNOWN_AVC_SKIPS)
