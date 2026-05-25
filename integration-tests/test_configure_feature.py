"""
:casecomponent: rhc
:requirement: RHSS-291300
:subsystemteam: rhel-sst-csi-client-tools
:caseautomation: Automated
:upstream: Yes

Integration tests for ``rhc configure features``

"""

import contextlib
import json
import os

import pytest

from utils import poll_until, prepare_args_for_connect

# CLI feature name -> key in ``rhc connect --format json`` features object
FEATURE_CLI_TO_CONNECT_JSON = {
    "content": "content",
    "analytics": "analytics",
    "remote-management": "remote_management",
}

CONNECT_FEATURES_PREFS_PATH = "/var/lib/rhc/rhc-connect-features-prefs.json"

# RHSM-managed repository file when ``rhsm.manage_repos`` / content feature is on.
REDHAT_REPO_FILE = "/etc/yum.repos.d/redhat.repo"

_CONFIGURE_FEATURES_STATUS_JSON_KEYS = frozenset({"connected", "features"})
_CONFIGURE_FEATURES_JSON_FEATURE_KEYS = frozenset(
    {"content", "analytics", "remote_management"}
)


@pytest.fixture(autouse=True)
def _cleanup_connect_features_prefs():
    """
    Remove the connect features preferences file.
    """
    yield
    with contextlib.suppress(FileNotFoundError):
        os.remove(CONNECT_FEATURES_PREFS_PATH)


def _assert_configure_features_status_json_shape(data: dict):
    assert isinstance(data, dict)
    assert _CONFIGURE_FEATURES_STATUS_JSON_KEYS == set(data.keys())
    assert isinstance(data["connected"], bool)
    feats = data["features"]
    assert isinstance(feats, dict)
    assert _CONFIGURE_FEATURES_JSON_FEATURE_KEYS == set(feats.keys())
    if data["connected"]:
        for _k, v in feats.items():
            assert set(v.keys()) == {"enabled", "description"}
            if "error" not in v:
                assert isinstance(v["enabled"], bool)
    else:
        for _k, v in feats.items():
            assert set(v.keys()) == {"description", "preference"}
            assert v["preference"] in ("enable", "skip")


def _preference_or_state(stdout: str, feature_id: str):
    """Return the second column (PREFERENCE or STATE) for a feature table row."""
    for line in stdout.splitlines():
        parts = line.split()
        if len(parts) >= 2 and parts[0] == feature_id:
            return parts[1]
    return None


@pytest.mark.tier1
@pytest.mark.parametrize(
    "status_format",
    [
        "text",
        pytest.param(
            "json",
            id="json",
        ),
    ],
    ids=["text", "json"],
)
def test_configure_features_status_not_connected_default_prefs(rhc, status_format):
    """
    :id: a1c2e3f4-5b6d-7e8f-90ab-cdef12345601
    :title: ``rhc configure features status`` shows default preferences when disconnected
    :description:
        While disconnected, preference defaults are ``enable`` for each feature,
        whether shown as a human table or as ``status --format json``.
    :parametrized: yes
    :tags: Tier 1
    :steps:
        1. Ensure the host is disconnected (``rhc disconnect``).
        2. Run ``rhc configure features status`` (text) or with ``--format json``.
    :expectedresults:
        1. Preconditions hold (disconnected).
        2. Exit code 0; text output has ``Not connected to Red Hat.`` and each
           feature ``enable``, or JSON has ``connected`` false and every feature
           ``preference`` ``enable``.
    """
    with contextlib.suppress(Exception):
        rhc.disconnect()
    if status_format == "text":
        result = rhc.run("configure", "features", "status", check=False)
        assert result.returncode == 0, result.stderr
        out = result.stdout
        assert "Not connected to Red Hat." in out
        assert "FEATURE" in out and "PREFERENCE" in out
        assert "STATE" not in out
        for feat in ("content", "analytics", "remote-management"):
            assert feat in out
            col = _preference_or_state(out, feat)
            assert col == "enable", f"expected default enable for {feat}, got {col!r}"
    else:
        res = rhc.run(
            "configure", "features", "status", "--format", "json", check=False
        )
        assert res.returncode == 0, res.stderr
        data = json.loads(res.stdout)
        _assert_configure_features_status_json_shape(data)
        assert data["connected"] is False
        for k in _CONFIGURE_FEATURES_JSON_FEATURE_KEYS:
            assert data["features"][k]["preference"] == "enable"


@pytest.mark.tier1
@pytest.mark.parametrize(
    "status_format",
    [
        "text",
        pytest.param(
            "json",
            id="json",
        ),
    ],
    ids=["text", "json"],
)
def test_configure_features_disable_updates_prefs_and_status(rhc, status_format):
    """
    :id: a1c2e3f4-5b6d-7e8f-90ab-cdef12345602
    :title: ``configure features disable analytics`` persists preferences when disconnected
    :description:
        Disabling ``analytics`` turns off dependent ``remote-management`` in the
        cache; human ``status`` or ``status --format json`` reflects the same
        preferences.
    :parametrized: yes
    :tags: Tier 1
    :steps:
        1. Disconnect the host.
        2. Run ``rhc configure features disable analytics``.
        3. Run ``rhc configure features status`` (text) or with ``--format json``.
    :expectedresults:
        1. Host is disconnected.
        2. Exit 0; message about ``remote-management`` depending on ``analytics``.
        3. ``content`` stays on; ``analytics`` and ``remote-management`` are off
           in prefs (table or JSON).
    """
    with contextlib.suppress(Exception):
        rhc.disconnect()
    d = rhc.run("configure", "features", "disable", "analytics", check=False)
    assert d.returncode == 0, d.stderr

    if status_format == "text":
        st = rhc.run("configure", "features", "status", check=False)
        assert st.returncode == 0, st.stderr
        body = st.stdout
        for feat, want in (
            ("content", "enable"),
            ("analytics", "skip"),
            ("remote-management", "skip"),
        ):
            actual = _preference_or_state(body, feat)
            assert actual == want, f"Feature {feat}: expected {want}, got {actual}"
    else:
        res = rhc.run(
            "configure", "features", "status", "--format", "json", check=False
        )
        assert res.returncode == 0, res.stderr
        data = json.loads(res.stdout)
        _assert_configure_features_status_json_shape(data)
        assert data["connected"] is False
        assert data["features"]["content"]["preference"] == "enable"
        assert data["features"]["analytics"]["preference"] == "skip"
        assert data["features"]["remote_management"]["preference"] == "skip"


@pytest.mark.tier1
def test_configure_features_enable_remote_management_pulls_prerequisites(rhc):
    """
    :id: a1c2e3f4-5b6d-7e8f-90ab-cdef12345603
    :title: ``configure features enable remote-management`` enables dependencies
    :description:
        On an unregistered system, enabling ``remote-management`` enables
        required features in the preference cache when they were off.
    :tags: Tier 1
    :steps:
        1. Disconnect; disable ``content`` (which also turns off
           ``remote-management`` in the cache).
        2. Run ``rhc configure features enable remote-management``.
        3. Run ``status``.
    :expectedresults:
        1. Host is disconnected.
        2. Success with notices for prerequisite features.
        3. All three features show preference ``enable``.
    """
    with contextlib.suppress(Exception):
        rhc.disconnect()
    for cmd in (
        ("configure", "features", "disable", "content"),
        ("configure", "features", "enable", "remote-management"),
    ):
        res = rhc.run(*cmd, check=False)
        assert res.returncode == 0, (cmd, res.stderr, res.stdout)

    st = rhc.run("configure", "features", "status", check=False)
    assert st.returncode == 0
    for feat in ("content", "analytics", "remote-management"):
        assert _preference_or_state(st.stdout, feat) == "enable"


@pytest.mark.tier1
@pytest.mark.parametrize(
    "args,expected_code,expected_substring",
    [
        pytest.param(
            ("configure", "features", "status", "--format", "json"),
            0,
            "",
            id="status-json-ok",
        ),
        (
            ("configure", "features", "enable", "not-a-real-feature"),
            65,
            "not found",
        ),
        (
            ("configure", "features", "disable", "not-a-real-feature"),
            65,
            "not found",
        ),
        (
            ("configure", "features", "enable"),
            64,
            "single FEATURE",
        ),
        (
            ("configure", "features", "disable", "content", "extra-arg"),
            64,
            "single FEATURE",
        ),
    ],
)
def test_configure_features_cli_errors(rhc, args, expected_code, expected_substring):
    """
    :id: a1c2e3f4-5b6d-7e8f-90ab-cdef12345604
    :title: ``configure features`` CLI validation (exit codes and error text)
    :description:
        Parametrized negative and success-path checks for ``configure features``
    :parametrized: yes
    :tags: Tier 1
    :steps:
        1. Ensure the host is disconnected (``rhc disconnect``).
        2. Run the parametrized ``rhc configure features`` command.
        3. Verify the exit code and, when applicable, error text in output.
    :expectedresults:
        1. Host is disconnected (precondition).
        2. Command completes with the parametrized exit code.
        3. For failure cases, stdout/stderr contains the expected substring;
          
    """
    with contextlib.suppress(Exception):
        rhc.disconnect()
    result = rhc.run(*args, check=False)
    assert result.returncode == expected_code
    if expected_substring:
        combined = result.stdout + result.stderr
        assert expected_substring.lower() in combined.lower()


@pytest.mark.tier1
def test_connect_honors_configure_features_preferences(
    external_candlepin, rhc, test_config
):
    """
    :id: a1c2e3f4-5b6d-7e8f-90ab-cdef12345605
    :title: ``rhc connect`` applies preferences set by ``configure features``
    :description:
        After ``rhc configure features disable analytics``, a normal connect
        (without ``--enable-feature`` / ``--disable-feature``) should load the
        cache and leave analytics (and dependent remote management) off.
    :tags: Tier 1
    :steps:
        1. Disconnect the host.
        2. Run ``rhc configure features disable analytics``.
        3. Run ``rhc connect`` with activation key and JSON output.
        4. Verify registration and connect JSON feature states.
    :expectedresults:
        1. Host is disconnected.
        2. Disable exits 0 and preferences are stored.
        3. Connect exits 0 and the system is registered.
        4. Connect JSON shows ``analytics`` and ``remote_management`` disabled,
           ``content`` enabled.
    """
    with contextlib.suppress(Exception):
        rhc.disconnect()

    d = rhc.run("configure", "features", "disable", "analytics", check=False)
    assert d.returncode == 0, d.stderr

    command_args = prepare_args_for_connect(
        test_config, auth="activation-key", output_format="json"
    )
    result = rhc.run("connect", *command_args, check=False)
    assert result.returncode == 0, (result.stdout, result.stderr)
    assert rhc.is_registered

    data = json.loads(result.stdout)
    features = data["features"]
    assert features[FEATURE_CLI_TO_CONNECT_JSON["analytics"]]["enabled"] is False
    assert (
        features[FEATURE_CLI_TO_CONNECT_JSON["remote-management"]]["enabled"] is False
    )
    assert features[FEATURE_CLI_TO_CONNECT_JSON["content"]]["enabled"] is True


@pytest.mark.tier1
def test_configure_features_status_connected(external_candlepin, rhc, test_config):
    """
    :id: a1c2e3f4-5b6d-7e8f-90ab-cdef12345606
    :title: ``rhc configure features status`` when connected shows live state
    :description:
        On a registered host, status uses the STATE column (enabled/disabled) instead
        of PREFERENCE.
    :tags: Tier 1
    :steps:
        1. Disconnect the host.
        2. Register the host.
        3. Run ``rhc configure features status`` (human output).
        4. Verify connected banner, column headers, and per-feature state.
    :expectedresults:
        1. Host is disconnected.
        2. Connect succeeds and the system is registered.
        3. Status exits 0.
        4. Output shows ``Connected to Red Hat.``, ``STATE`` (not ``PREFERENCE``),
           and each feature ``enabled``.
    """
    with contextlib.suppress(Exception):
        rhc.disconnect()

    rhc.connect(
        activationkey=test_config.get("candlepin.activation_keys")[0],
        org=test_config.get("candlepin.org"),
    )
    assert rhc.is_registered

    st = rhc.run("configure", "features", "status", check=False)
    assert st.returncode == 0, st.stderr
    out = st.stdout
    assert "Connected to Red Hat." in out
    assert "STATE" in out
    assert "PREFERENCE" not in out
    for feat in ("content", "analytics", "remote-management"):
        assert feat in out
        state = _preference_or_state(out, feat)
        assert state == "enabled", f"{feat}: expected enabled, got {state!r}"


@pytest.mark.tier1
def test_configure_features_disable_after_connect(
    external_candlepin, rhc, test_config
):
    """
    :id: a1c2e3f4-5b6d-7e8f-90ab-cdef12345607
    :title: ``configure features disable`` updates live ``STATE`` after ``rhc connect``
    :description:
        On a registered host, ``disable analytics`` mutates live feature state;
        ``configure features status`` shows ``STATE`` (not preference-file behavior).
        Disabling ``analytics`` also turns off dependent ``remote-management``.
    :tags: Tier 1
    :steps:
        1. Disconnect, then register the host.
        2. Run ``rhc configure features disable analytics``.
        3. Run ``rhc configure features status`` and verify live ``STATE`` values.
    :expectedresults:
        1. System is registered after connect.
        2. Disable exits 0.
        3. ``content`` enabled; ``analytics`` and ``remote-management`` disabled.
    """
    with contextlib.suppress(Exception):
        rhc.disconnect()
    rhc.connect(
        activationkey=test_config.get("candlepin.activation_keys")[0],
        org=test_config.get("candlepin.org"),
    )
    assert rhc.is_registered

    d = rhc.run("configure", "features", "disable", "analytics", check=False)
    assert d.returncode == 0, d.stderr

    st = rhc.run("configure", "features", "status", check=False)
    assert st.returncode == 0, st.stderr
    out = st.stdout
    assert "Connected to Red Hat." in out
    assert _preference_or_state(out, "content") == "enabled"
    assert _preference_or_state(out, "analytics") == "disabled"
    assert _preference_or_state(out, "remote-management") == "disabled"


@pytest.mark.tier1
def test_configure_features_enable_after_connect(
    external_candlepin, rhc, test_config
):
    """
    :id: a1c2e3f4-5b6d-7e8f-90ab-cdef12345609
    :title: ``configure features enable`` updates live ``STATE`` after ``rhc connect``
    :description:
        On a registered host with ``analytics`` disabled, ``enable analytics`` turns
        analytics back on in live ``STATE`` without re-enabling dependent
        ``remote-management``.
    :tags: Tier 1
    :steps:
        1. Disconnect the host.
        2. Run ``rhc configure features disable analytics`` (preference cache).
        3. Run ``rhc connect`` with activation key.
        4. Run ``rhc configure features enable analytics`` on the registered host.
        5. Run ``rhc configure features status`` and verify live ``STATE`` values.
    :expectedresults:
        1. Host is disconnected.
        2. Disable analytics exits 0.
        3. Connect exits 0; system is registered with analytics off from preferences.
        4. Enable analytics exits 0.
        5. ``content`` and ``analytics`` enabled; ``remote-management`` still disabled.
    """
    with contextlib.suppress(Exception):
        rhc.disconnect()

    d = rhc.run("configure", "features", "disable", "analytics", check=False)
    assert d.returncode == 0, d.stderr

    rhc.connect(
        activationkey=test_config.get("candlepin.activation_keys")[0],
        org=test_config.get("candlepin.org"),
    )
    assert rhc.is_registered

    e = rhc.run("configure", "features", "enable", "analytics", check=False)
    assert e.returncode == 0, e.stderr

    st = rhc.run("configure", "features", "status", check=False)
    assert st.returncode == 0, st.stderr
    out = st.stdout
    assert _preference_or_state(out, "content") == "enabled"
    assert _preference_or_state(out, "analytics") == "enabled"
    assert _preference_or_state(out, "remote-management") == "disabled"


@pytest.mark.tier1
def test_configure_features_status_json_when_connected(
    external_candlepin, rhc, test_config
):
    """
    :id: c3d4e5f6-7a8b-9c0d-1e2f-3a4b5c6d7e05
    :title: ``configure features status --format json`` when connected 
    :description:
        On a registered host, ``configure features status --format json`` reports
        ``connected`` true and live boolean state for each feature (all enabled
        after a default connect).
    :tags: Tier 1
    :steps:
        1. Disconnect the host.
        2. Register the host.
        3. Run ``rhc configure features status --format json``.
        4. Parse JSON and verify schema and feature booleans.
    :expectedresults:
        1. Host is disconnected.
        2. system is registered.
        3. Status JSON command exits 0.
        4. Document has ``connected`` true; each feature ``enabled`` true with
           stable keys.
    """
    with contextlib.suppress(Exception):
        rhc.disconnect()
    rhc.connect(
        activationkey=test_config.get("candlepin.activation_keys")[0],
        org=test_config.get("candlepin.org"),
    )
    assert rhc.is_registered
    res = rhc.run("configure", "features", "status", "--format", "json", check=False)
    assert res.returncode == 0, res.stderr
    data = json.loads(res.stdout)
    _assert_configure_features_status_json_shape(data)
    assert data["connected"] is True
    for k in _CONFIGURE_FEATURES_JSON_FEATURE_KEYS:
        assert data["features"][k]["enabled"] is True


@pytest.mark.tier1
@pytest.mark.parametrize(
    "status_format",
    [
        "text",
        pytest.param(
            "json",
            id="json",
        ),
    ],
    ids=["text", "json"],
)
def test_configure_features_enable_content_after_disable(rhc, status_format):
    """
    :id: c3d4e5f6-7a8b-9c0d-1e2f-3a4b5c6d7e06
    :title: ``configure features enable content`` restores prefs after ``disable content``
    :description:
        On a disconnected host, disabling then re-enabling ``content`` updates the
        preference cache; ``status`` (text or JSON) shows ``content`` on,
        ``remote-management`` still off until explicitly enabled.
    :parametrized: yes
    :tags: Tier 1
    :steps:
        1. Disconnect the host.
        2. Run ``rhc configure features disable content``.
        3. Run ``rhc configure features enable content``.
        4. Run ``rhc configure features status`` (text) or with ``--format json``.
    :expectedresults:
        1. Host is disconnected.
        2. Disable exits 0; preference file exists.
        3. Enable content exits 0.
        4. ``content`` and ``analytics`` on; ``remote-management`` off (table
           or JSON ``preference`` ``skip``).
    """
    with contextlib.suppress(Exception):
        rhc.disconnect()
    d = rhc.run("configure", "features", "disable", "content", check=False)
    assert d.returncode == 0, d.stderr
    assert os.path.isfile(CONNECT_FEATURES_PREFS_PATH)
    e = rhc.run("configure", "features", "enable", "content", check=False)
    assert e.returncode == 0, e.stderr
    if status_format == "text":
        st = rhc.run("configure", "features", "status", check=False)
        assert st.returncode == 0, st.stderr
        out = st.stdout
        assert _preference_or_state(out, "content") == "enable"
        assert _preference_or_state(out, "analytics") == "enable"
        assert _preference_or_state(out, "remote-management") == "skip"
    else:
        res = rhc.run(
            "configure", "features", "status", "--format", "json", check=False
        )
        assert res.returncode == 0, res.stderr
        data = json.loads(res.stdout)
        _assert_configure_features_status_json_shape(data)
        assert data["connected"] is False
        assert data["features"]["content"]["preference"] == "enable"
        assert data["features"]["analytics"]["preference"] == "enable"
        assert data["features"]["remote_management"]["preference"] == "skip"


@pytest.mark.tier1
def test_configure_features_prefs_removed_after_connect(
    external_candlepin, rhc, test_config
):
    """
    :id: c3d4e5f6-7a8b-9c0d-1e2f-3a4b5c6d7e0e
    :title: Preference file is removed after successful ``rhc connect``
    :description:
        Preferences written under ``/var/lib/rhc/rhc-connect-features-prefs.json``
        before connect are consumed and the file is deleted after a successful
        ``rhc connect``.
    :tags: Tier 1
    :steps:
        1. Disconnect the host.
        2. Run ``rhc configure features disable analytics``.
        3. Confirm the preference file exists.
        4. Run ``rhc connect`` with activation key.
        5. Verify the preference file is absent.
    :expectedresults:
        1. Host is disconnected.
        2. Disable exits 0.
        3. ``rhc-connect-features-prefs.json`` is present.
        4. Connect exits 0 and the system is registered.
        5. Preference file is no longer on disk.
    """
    with contextlib.suppress(Exception):
        rhc.disconnect()
    d = rhc.run("configure", "features", "disable", "analytics", check=False)
    assert d.returncode == 0, d.stderr
    assert os.path.isfile(CONNECT_FEATURES_PREFS_PATH)
    command_args = prepare_args_for_connect(
        test_config, auth="activation-key", output_format="json"
    )
    result = rhc.run("connect", *command_args, check=False)
    assert result.returncode == 0, (result.stdout, result.stderr)
    assert not os.path.isfile(CONNECT_FEATURES_PREFS_PATH)




@pytest.mark.tier2
@pytest.mark.skip(reason="Known issue: redhat.repo not created under test, ref: CCT-2436")
def test_configure_features_content_toggle_redhat_repo_after_connect(
    external_candlepin, rhc, test_config
):
    """
    :id: a1c2e3f4-5b6d-7e8f-90ab-cdef12345608
    :title: Disabling content on a registered host removes ``redhat.repo``; enabling restores it
    :description:
        With content enabled after ``rhc connect``, Subscription Management should
        maintain ``/etc/yum.repos.d/redhat.repo``. Turning the content feature off
        removes that repo file; turning it back on restores it.
    :tags: Tier 2
    :steps:
        1. Disconnect, then register the host (content enabled).
        2. Verify ``redhat.repo`` exists
        3. Run ``rhc configure features disable content``.
        4. Verify ``redhat.repo`` is absent.
        5. Run ``rhc configure features enable content``.
        6. Verify ``redhat.repo`` exists again.
    :expectedresults:
        1. Connect succeeds with content enabled.
        2. ``redhat.repo`` exists.
        3. ``disable content`` exits 0.
        4. Repo file is removed after content is disabled.
        5. ``enable content`` exits 0.
        6. Repo file is present again after content is re-enabled.
    """
    with contextlib.suppress(Exception):
        rhc.disconnect()

    command_args = prepare_args_for_connect(
        test_config, auth="activation-key", output_format="json"
    )
    result = rhc.run("connect", *command_args, check=False)
    assert result.returncode == 0, (result.stdout, result.stderr)
    assert rhc.is_registered

    connect_data = json.loads(result.stdout)
    assert (
        connect_data["features"]["content"]["enabled"] is True
    ), "content must be enabled after connect for this repository check"

    if not poll_until(lambda: os.path.isfile(REDHAT_REPO_FILE), timeout_s=60):
        pytest.skip(
            f"{REDHAT_REPO_FILE} did not appear after connect; "
            "environment may not provide attachable yum content"
        )

    d = rhc.run("configure", "features", "disable", "content", check=False)
    assert d.returncode == 0, d.stderr

    assert poll_until(lambda: not os.path.isfile(REDHAT_REPO_FILE), timeout_s=60), (
        f"expected {REDHAT_REPO_FILE} to be removed after disabling content, "
        "but it still exists"
    )

    e = rhc.run("configure", "features", "enable", "content", check=False)
    assert e.returncode == 0, e.stderr

    assert poll_until(
        lambda: os.path.isfile(REDHAT_REPO_FILE), timeout_s=60
    ), f"expected {REDHAT_REPO_FILE} to reappear after enabling content"


@pytest.mark.tier1
def test_configure_features_disable_idempotent_json(rhc):
    """
    :id: c3d4e5f6-7a8b-9c0d-1e2f-3a4b5c6d7e13
    :title: Repeated ``disable analytics`` yields same JSON status (idempotent)
    :description:
        Running ``disable analytics`` twice on a disconnected host leaves JSON
        status unchanged (no unintended preference drift).
    :tags: Tier 1
    :steps:
        1. Disconnect the host.
        2. Run ``rhc configure features disable analytics``.
        3. Run ``rhc configure features status --format json`` (first snapshot).
        4. Run ``rhc configure features disable analytics`` again.
        5. Run ``rhc configure features status --format json`` (second snapshot).
        6. Compare the two JSON documents.
    :expectedresults:
        1. Host is disconnected.
        2. First disable exits 0.
        3. First JSON status is captured.
        4. Second disable exits 0.
        5. Second JSON status is captured.
        6. First and second JSON are equal.
    """
    with contextlib.suppress(Exception):
        rhc.disconnect()
    assert (
        rhc.run("configure", "features", "disable", "analytics", check=False).returncode
        == 0
    )
    a = json.loads(
        rhc.run(
            "configure", "features", "status", "--format", "json", check=False
        ).stdout
    )
    assert (
        rhc.run("configure", "features", "disable", "analytics", check=False).returncode
        == 0
    )
    b = json.loads(
        rhc.run(
            "configure", "features", "status", "--format", "json", check=False
        ).stdout
    )
    assert a == b
