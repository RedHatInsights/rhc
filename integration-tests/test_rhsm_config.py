"""
:casecomponent: rhc
:requirement: RHSS-291300
:subsystemteam: rhel-sst-csi-client-tools
:caseautomation: Automated
:upstream: Yes

Integration tests for writing explicit rhc.conf keys into rhsm.conf.
"""

import configparser
import contextlib
from pathlib import Path

import pytest

from utils.constants import CONNECT_FEATURES_PREFS_PATH, EXIT_CODE_SOFTWARE

RHC_CONF = Path("/etc/rhc/rhc.conf")
RHSM_CONF = Path("/etc/rhsm/rhsm.conf")
PREFS = Path(CONNECT_FEATURES_PREFS_PATH)
PACKAGED_RHC_CONF = Path(__file__).resolve().parents[1] / "data" / "config" / "rhc.conf"

SATELLITE_RHC_CONF = """
[api.subscriptions]
uri = "https://satellite.example.com/subscription"
ca-path = "/etc/rhsm/ca/katello-server-ca.pem"
"""

INVALID_SUBSCRIPTIONS_URI = """
[api.subscriptions]
uri = "http://satellite.example.com/subscription"
"""

RPM_CONTENT_RHC_CONF = """
[api.rpm-content]
uri = "https://satellite.example.com/pulp/content"
tls-verify = false
ca-path = "/etc/rhsm/ca/katello-server-ca.pem"
"""

INVALID_RPM_CONTENT_URI = """
[api.rpm-content]
uri = "cdn.example/content"
"""


def _rhsm_value(section, option):
    parser = configparser.ConfigParser(interpolation=None)
    parser.read(RHSM_CONF)
    if not parser.has_option(section, option):
        return None
    return parser.get(section, option).strip().strip('"')


def _save_file(path):
    if not path.exists():
        return None
    return {"data": path.read_bytes(), "mode": path.stat().st_mode}


def _restore_file(path, saved):
    if saved is None:
        path.unlink(missing_ok=True)
        return
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_bytes(saved["data"])
    path.chmod(saved["mode"])


@pytest.fixture
def restored_rhc_configuration():
    """Put rhc.conf, rhsm.conf, and the feature preference cache back."""
    saved = {
        "rhsm": _save_file(RHSM_CONF),
        "rhc": _save_file(RHC_CONF),
        "prefs": _save_file(PREFS),
    }
    try:
        yield
    finally:
        _restore_file(RHSM_CONF, saved["rhsm"])
        _restore_file(RHC_CONF, saved["rhc"])
        _restore_file(PREFS, saved["prefs"])


def _disconnect(rhc):
    with contextlib.suppress(Exception):
        rhc.disconnect()
    assert not rhc.is_registered


@pytest.mark.tier1
def test_commented_rhc_conf_leaves_rhsm_conf_unchanged(rhc, restored_rhc_configuration):
    """
    :id: 7c1a9e24-6b0d-4f3a-9c55-2a6e8d0b1f40
    :title: A commented rhc.conf does not change rhsm.conf
    :description:
        ``rhc configure features enable`` on an unregistered system reads
        rhc.conf. When every registration and proxy key is commented, it
        makes no Config.Set calls.
    :tags: Tier 1
    :steps:
        1. Disconnect.
        2. Install the packaged, fully commented rhc.conf.
        3. Run ``rhc configure features enable content``.
    :expectedresults:
        1. The system is not registered.
        2. rhc.conf is the packaged file.
        3. The command succeeds and rhsm.conf is byte-for-byte unchanged.
    """
    _disconnect(rhc)
    RHC_CONF.parent.mkdir(parents=True, exist_ok=True)
    RHC_CONF.write_text(PACKAGED_RHC_CONF.read_text())
    before = RHSM_CONF.read_bytes()

    res = rhc.run("configure", "features", "enable", "content", check=False)

    assert res.returncode == 0, (res.stdout, res.stderr)
    assert RHSM_CONF.read_bytes() == before
    assert not rhc.is_registered


@pytest.mark.tier1
def test_features_enable_writes_satellite_registration_target(rhc, restored_rhc_configuration):
    """
    :id: 4e8b2c71-1d5a-4a9e-8f66-0c3b7a9d2e18
    :title: features enable writes the explicit Satellite registration target
    :description:
        On an unregistered system, an explicit ``[api.subscriptions]`` uri
        and ca-path are written to rhsm.conf. Keys that rhc.conf does not
        set, including server.insecure and rhsm.baseurl, stay as they were.
    :tags: Tier 1
    :steps:
        1. Disconnect and record server.insecure, server.proxy_hostname, and
           rhsm.baseurl.
        2. Write a Satellite ``[api.subscriptions]`` block to rhc.conf.
        3. Run ``rhc configure features enable content``.
    :expectedresults:
        1. The system is not registered.
        2. rhc.conf contains only the Satellite uri and ca-path.
        3. server.hostname, server.port, server.prefix, and rhsm.repo_ca_cert
           match that block. server.insecure, server.proxy_hostname, and
           rhsm.baseurl are unchanged, and the system is still not registered.
    """
    _disconnect(rhc)
    insecure = _rhsm_value("server", "insecure")
    proxy_hostname = _rhsm_value("server", "proxy_hostname")
    baseurl = _rhsm_value("rhsm", "baseurl")
    RHC_CONF.parent.mkdir(parents=True, exist_ok=True)
    RHC_CONF.write_text(SATELLITE_RHC_CONF)

    res = rhc.run("configure", "features", "enable", "content", check=False)

    assert res.returncode == 0, (res.stdout, res.stderr)
    assert _rhsm_value("server", "hostname") == "satellite.example.com"
    assert _rhsm_value("server", "port") == "443"
    assert _rhsm_value("server", "prefix") == "/subscription"
    assert _rhsm_value("rhsm", "repo_ca_cert") == "/etc/rhsm/ca/katello-server-ca.pem"
    assert _rhsm_value("server", "insecure") == insecure
    assert _rhsm_value("server", "proxy_hostname") == proxy_hostname
    assert _rhsm_value("rhsm", "baseurl") == baseurl
    assert not rhc.is_registered


@pytest.mark.tier1
def test_connect_invalid_subscriptions_uri_does_not_register(rhc, restored_rhc_configuration):
    """
    :id: 9a0d6f33-c4e2-4b71-a158-6d2e0c8b4f77
    :title: An unusable subscriptions URI fails connect before Register
    :description:
        ``rhc connect`` rejects an explicit http subscriptions URI before
        it calls Config.Set or Register, and leaves rhsm.conf unchanged.
    :tags: Tier 1
    :steps:
        1. Disconnect.
        2. Write ``uri = "http://satellite.example.com/subscription"``.
        3. Run ``rhc connect`` with an activation key.
    :expectedresults:
        1. The system is not registered.
        2. rhc.conf contains the http URI.
        3. The command fails, rhsm.conf is unchanged, and the system is
           still not registered.
    """
    _disconnect(rhc)
    before = RHSM_CONF.read_bytes()
    RHC_CONF.parent.mkdir(parents=True, exist_ok=True)
    RHC_CONF.write_text(INVALID_SUBSCRIPTIONS_URI)

    res = rhc.run(
        "connect",
        "--organization",
        "unused",
        "--activation-key",
        "unused",
        check=False,
    )

    combined = res.stdout + res.stderr
    assert res.returncode != 0, combined
    assert "api.subscriptions.uri" in combined
    assert RHSM_CONF.read_bytes() == before
    assert not rhc.is_registered


@pytest.mark.tier1
def test_features_enable_invalid_subscriptions_uri_fails(rhc, restored_rhc_configuration):
    """
    :id: b2e7c814-5f0a-4d93-9e21-7c4a1b6d8e05
    :title: features enable fails before Config.Set when the URI is unusable
    :description:
        ``rhc configure features enable`` on an unregistered system returns
        a software error for an explicit http subscriptions URI and does
        not change rhsm.conf.
    :tags: Tier 1
    :steps:
        1. Disconnect.
        2. Write ``uri = "http://satellite.example.com/subscription"``.
        3. Run ``rhc configure features enable content``.
    :expectedresults:
        1. The system is not registered.
        2. rhc.conf contains the http URI.
        3. The command exits 70, rhsm.conf is unchanged, and the system
           is still not registered.
    """
    _disconnect(rhc)
    before = RHSM_CONF.read_bytes()
    RHC_CONF.parent.mkdir(parents=True, exist_ok=True)
    RHC_CONF.write_text(INVALID_SUBSCRIPTIONS_URI)

    res = rhc.run("configure", "features", "enable", "content", check=False)

    assert res.returncode == EXIT_CODE_SOFTWARE, (res.stdout, res.stderr)
    assert RHSM_CONF.read_bytes() == before
    assert not rhc.is_registered


@pytest.mark.tier1
def test_features_enable_writes_rpm_content_baseurl(rhc, restored_rhc_configuration):
    """
    :id: 3f6a1c90-8d24-4e17-b5a2-9c0e7d4f18ab
    :title: features enable writes an explicit RPM content URI to rhsm.baseurl
    :description:
        On an unregistered system, an explicit ``[api.rpm-content]`` uri is
        written to rhsm.baseurl. tls-verify and ca-path in that section are
        not written: server.insecure and rhsm.repo_ca_cert belong to
        ``[api.subscriptions]``.
    :tags: Tier 1
    :steps:
        1. Disconnect and record server.hostname, server.insecure, and
           rhsm.repo_ca_cert.
        2. Write an ``[api.rpm-content]`` block with a uri, tls-verify, and
           ca-path.
        3. Run ``rhc configure features enable content``.
    :expectedresults:
        1. The system is not registered.
        2. rhc.conf contains only the RPM content block.
        3. rhsm.baseurl matches the uri. server.hostname, server.insecure,
           and rhsm.repo_ca_cert are unchanged, and the system is still
           not registered.
    """
    _disconnect(rhc)
    hostname = _rhsm_value("server", "hostname")
    insecure = _rhsm_value("server", "insecure")
    repo_ca_cert = _rhsm_value("rhsm", "repo_ca_cert")
    RHC_CONF.parent.mkdir(parents=True, exist_ok=True)
    RHC_CONF.write_text(RPM_CONTENT_RHC_CONF)

    res = rhc.run("configure", "features", "enable", "content", check=False)

    assert res.returncode == 0, (res.stdout, res.stderr)
    assert _rhsm_value("rhsm", "baseurl") == "https://satellite.example.com/pulp/content"
    assert _rhsm_value("server", "hostname") == hostname
    assert _rhsm_value("server", "insecure") == insecure
    assert _rhsm_value("rhsm", "repo_ca_cert") == repo_ca_cert
    assert not rhc.is_registered


@pytest.mark.tier1
def test_features_enable_invalid_rpm_content_uri_fails(rhc, restored_rhc_configuration):
    """
    :id: 6d1b8e45-2a73-4c09-8f16-5e9a0b3c7d22
    :title: features enable fails before Config.Set when the RPM content URI is unusable
    :description:
        ``rhc configure features enable`` on an unregistered system returns
        a software error for an explicit RPM content URI with no scheme and
        does not change rhsm.conf.
    :tags: Tier 1
    :steps:
        1. Disconnect.
        2. Write ``uri = "cdn.example/content"`` under ``[api.rpm-content]``.
        3. Run ``rhc configure features enable content``.
    :expectedresults:
        1. The system is not registered.
        2. rhc.conf contains the URI with no scheme.
        3. The command exits 70, rhsm.conf is unchanged, and the system
           is still not registered.
    """
    _disconnect(rhc)
    before = RHSM_CONF.read_bytes()
    RHC_CONF.parent.mkdir(parents=True, exist_ok=True)
    RHC_CONF.write_text(INVALID_RPM_CONTENT_URI)

    res = rhc.run("configure", "features", "enable", "content", check=False)

    combined = res.stdout + res.stderr
    assert res.returncode == EXIT_CODE_SOFTWARE, combined
    assert "api.rpm-content.uri" in combined
    assert RHSM_CONF.read_bytes() == before
    assert not rhc.is_registered
