from rhc_selinux import avc_skips_for_test


def test_known_rhsmcertd_avcs_are_skipped():
    expected = {
        "fields": {
            "subj": "system_u:system_r:rhsmcertd_t:s0",
        }
    }

    assert expected in avc_skips_for_test("test_any_rhc_case")


def test_known_rhel10_auditctl_avc_is_skipped():
    expected = {
        "fields": {
            "subj": "system_u:system_r:auditctl_t:s0",
            "permission": "read",
            "obj": "system_u:object_r:auditd_log_t:s0",
        }
    }

    assert expected in avc_skips_for_test("test_any_rhc_case")
