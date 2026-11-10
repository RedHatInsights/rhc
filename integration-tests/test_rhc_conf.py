"""
:casecomponent: rhc
:requirement: RHSS-291300
:subsystemteam: rhel-sst-csi-client-tools
:caseautomation: Automated
:upstream: Yes
"""

import shlex
from pathlib import Path

import pytest

from utils.constants import EXIT_CODE_USAGE, LOG_FILE_PATH


@pytest.fixture
def rhc_conf_files(rhc):
    """Restore both configuration files in place, preserving permissions and labels."""
    rhc_path = Path("/etc/rhc/rhc.conf")
    rhsm_path = Path("/etc/rhsm/rhsm.conf")
    rhc_original = rhc_path.read_bytes()
    rhsm_original = rhsm_path.read_bytes()
    try:
        yield rhc_path, rhsm_path
    finally:
        try:
            rhc_path.write_bytes(rhc_original)
        finally:
            rhsm_path.write_bytes(rhsm_original)


@pytest.mark.tier1
@pytest.mark.parametrize(
    "hostname, certificate, rhc_extra, expected_ingress, expected_inventory",
    [
        pytest.param(
            "subscription.rhsm.redhat.com", "redhat-uep.pem",
            "",
            ("https://cert.console.redhat.com/api/ingress/v1/upload", "default"),
            ("https://cert.console.redhat.com/api/inventory/v1/hosts", "default"),
            id="hosted",
        ),
        pytest.param(
            "subscription.rhsm.stage.redhat.com", "redhat-uep.pem", "",
            ("https://cert.console.stage.redhat.com/api/ingress/v1/upload", "rhsm.conf"),
            ("https://cert.console.redhat.com/api/inventory/v1/hosts", "default"),
            id="stage",
        ),
        pytest.param(
            "satellite.example.com", "katello-server-ca.pem",
            "",
            ("https://satellite.example.com/redhat_access/r/insights/platform/ingress/v1/upload", "rhsm.conf"),
            ("https://satellite.example.com/redhat_access/r/insights/platform/inventory/v1", "rhsm.conf"),
            id="satellite",
        ),
        pytest.param(
            "satellite.example.com", "katello-server-ca.pem",
            '\n[api.insights.ingress]\nuri = "https://custom.example.com:8443/upload?target=custom"\n',
            ("https://custom.example.com:8443/upload?target=custom", "rhc.conf"),
            ("https://satellite.example.com/redhat_access/r/insights/platform/inventory/v1", "rhsm.conf"),
            id="explicit-ingress",
        ),
        pytest.param(
            "satellite.example.com", "katello-server-ca.pem",
            '\n[api.subscriptions]\nuri = "https://subscription.rhsm.stage.redhat.com/subscription"\n',
            ("https://cert.console.stage.redhat.com/api/ingress/v1/upload", "rhc.conf"),
            ("https://cert.console.redhat.com/api/inventory/v1/hosts", "default"),
            id="effective-stage-target",
        ),
    ],
)
def test_insights_endpoints_from_configuration(
    rhc, rhc_conf_files, hostname, certificate, rhc_extra,
    expected_ingress, expected_inventory,
):
    """
    :id: 8927f853-602e-441a-97f4-b9197d636a0f
    :title: Verify effective Insights endpoints through rhc startup
    :description:
        Load representative hosted, stage, and Satellite configurations through
        the installed rhc command. Verify endpoint URLs and their sources,
        including registration and endpoint overrides.
    :parametrized: yes
    :tags: Tier 1
    :steps:
        1. Back up rhc.conf and rhsm.conf, then write the fixture configurations.
        2. Run rhc with debug logging and a collector info command missing its ID.
        3. Check the newly written endpoint log entries and restore both files.
    :expectedresults:
        1. The representative configuration is written at the standard paths.
        2. Startup loads configuration, then exits with usage code 64.
        3. Both endpoint URLs and sources match; original files are restored.
    """
    rhc_path, rhsm_path = rhc_conf_files
    rhc_path.write_text(
        "[compatibility]\n#interpret-legacy-configurations = true\n"
        "\n[http.proxy]\n#uri = \"\"\n" + rhc_extra
    )
    rhsm_path.write_text(f"""[server]
hostname = {hostname}
port = 443
prefix = /subscription
insecure = 0

[rhsm]
ca_cert_dir = /etc/rhsm/ca/
repo_ca_cert = %(ca_cert_dir)s{certificate}
""")
    log_path = Path(LOG_FILE_PATH)
    start_pos = log_path.stat().st_size if log_path.exists() else 0

    result = rhc.run("--log-level", "debug", "collector", "info", check=False)

    assert result.returncode == EXIT_CODE_USAGE, result.stdout + result.stderr
    with log_path.open("rb") as log_file:
        log_file.seek(start_pos)
        content = log_file.read().decode("utf-8")
    entries = [
        dict(field.split("=", 1) for field in shlex.split(line))
        for line in content.splitlines() if line.strip()
    ]
    for endpoint, (uri, source) in (
        ("ingress", expected_ingress), ("inventory", expected_inventory),
    ):
        assert any(
            entry.get("msg") == f"effective api.insights.{endpoint}"
            and entry.get("source") == source
            and entry.get("uri") == uri
            for entry in entries
        ), content


def _effective_config_log_entries(rhc, rhc_path, rhsm_path, rhc_extra):
    """Write configs, run rhc with debug logging, and return parsed log entries.

    Uses a stock hosted rhsm.conf and appends rhc_extra to a minimal rhc.conf,
    then runs an incomplete 'collector info' command so rhc loads and logs its
    effective configuration during startup before exiting with the usage code.
    """
    rhc_path.write_text(
        "[compatibility]\n#interpret-legacy-configurations = true\n"
        "\n[http.proxy]\n#uri = \"\"\n" + rhc_extra
    )
    rhsm_path.write_text(
        "[server]\n"
        "hostname = subscription.rhsm.redhat.com\n"
        "port = 443\n"
        "prefix = /subscription\n"
        "insecure = 0\n"
        "\n[rhsm]\n"
        "ca_cert_dir = /etc/rhsm/ca/\n"
        "repo_ca_cert = %(ca_cert_dir)sredhat-uep.pem\n"
    )
    log_path = Path(LOG_FILE_PATH)
    start_pos = log_path.stat().st_size if log_path.exists() else 0

    result = rhc.run("--log-level", "debug", "collector", "info", check=False)

    assert result.returncode == EXIT_CODE_USAGE, result.stdout + result.stderr
    with log_path.open("rb") as log_file:
        log_file.seek(start_pos)
        content = log_file.read().decode("utf-8")

    entries = [
        dict(field.split("=", 1) for field in shlex.split(line))
        for line in content.splitlines() if line.strip()
    ]
    return entries, content


@pytest.mark.tier1
def test_insights_ingress_client_cert_from_configuration(rhc, rhc_conf_files):
    """
    :id: 2f1c4b0e-8d3a-4e7c-9a21-6f0b5d4c8e13
    :title: Verify Insights ingress client cert/key through rhc startup
    :description:
        rhc-collector authenticates to the Insights ingress endpoint with the
        consumer identity certificate and key managed by subscription-manager.
        ADR-013 keeps both paths out of rhc.conf, so verify the effective
        cert-path and key-path are the consumer identity ones.
    :tags: Tier 1
    :steps:
        1. Back up rhc.conf and rhsm.conf, then write the fixture configuration.
        2. Run rhc with debug logging and an incomplete collector command.
        3. Check the effective ingress log entry and restore both files.
    :expectedresults:
        1. The configuration is written at the standard paths.
        2. Startup loads configuration, then exits with usage code 64.
        3. The logged cert-path and key-path match; original files are restored.
    """
    rhc_path, rhsm_path = rhc_conf_files
    entries, content = _effective_config_log_entries(rhc, rhc_path, rhsm_path, "")
    assert any(
        entry.get("msg") == "effective api.insights.ingress"
        and entry.get("cert-path") == "/etc/pki/consumer/cert.pem"
        and entry.get("key-path") == "/etc/pki/consumer/key.pem"
        for entry in entries
    ), content


@pytest.mark.tier1
def test_insights_ingress_ca_path_and_tls_verify_from_configuration(rhc, rhc_conf_files):
    """
    :id: 5c8e2a71-3d94-4b60-9f12-7a0e4c1b9d35
    :title: Verify Insights ingress ca-path and tls-verify through rhc startup
    :description:
        Unlike the fixed client identity, the Insights ingress CA bundle and
        TLS verification are configurable in rhc.conf. Verify an explicit
        ca-path and tls-verify are reflected in the effective ingress endpoint.
    :tags: Tier 1
    :steps:
        1. Back up rhc.conf and rhsm.conf, then write an [api.insights.ingress]
           block with an explicit uri, tls-verify, and ca-path.
        2. Run rhc with debug logging and an incomplete collector command.
        3. Check the effective ingress log entry and restore both files.
    :expectedresults:
        1. The configuration is written at the standard paths.
        2. Startup loads configuration, then exits with usage code 64.
        3. The logged ca-path and tls-verify match; original files are restored.
    """
    rhc_path, rhsm_path = rhc_conf_files
    rhc_extra = (
        '\n[api.insights.ingress]\n'
        'uri = "https://ingress.example.com/api/ingress/v1/upload"\n'
        'tls-verify = false\n'
        'ca-path = "/etc/pki/custom-ca.pem"\n'
    )
    entries, content = _effective_config_log_entries(rhc, rhc_path, rhsm_path, rhc_extra)
    assert any(
        entry.get("msg") == "effective api.insights.ingress"
        and entry.get("ca-path") == "/etc/pki/custom-ca.pem"
        and entry.get("tls-verify") == "false"
        for entry in entries
    ), content


@pytest.mark.tier1
@pytest.mark.parametrize(
    "rhc_extra, connect, request_, idle",
    [
        pytest.param("", "30s", "3m0s", "2m0s", id="default-timeouts"),
        pytest.param(
            '\n[http.timeout]\nconnect = 5\nrequest = 60\nidle = 15\n',
            "5s", "1m0s", "15s",
            id="explicit-timeouts",
        ),
    ],
)
def test_http_timeouts_from_configuration(
    rhc, rhc_conf_files, rhc_extra, connect, request_, idle,
):
    """
    :id: b4f1d7a2-6c05-49e8-8a3b-2e9f0d6c1a84
    :title: Verify HTTP timeouts through rhc startup
    :description:
        rhc resolves connect, request, and idle HTTP timeouts from rhc.conf's
        [http.timeout] section. Verify the effective timeouts, both the
        defaults and explicit rhc.conf overrides.
    :parametrized: yes
    :tags: Tier 1
    :steps:
        1. Back up rhc.conf and rhsm.conf, then write the fixture configuration.
        2. Run rhc with debug logging and an incomplete collector command.
        3. Check the effective http timeouts log entry and restore both files.
    :expectedresults:
        1. The configuration is written at the standard paths.
        2. Startup loads configuration, then exits with usage code 64.
        3. The logged connect, request, and idle timeouts match; original
           files are restored.
    """
    rhc_path, rhsm_path = rhc_conf_files
    entries, content = _effective_config_log_entries(rhc, rhc_path, rhsm_path, rhc_extra)
    assert any(
        entry.get("msg") == "effective http timeouts"
        and entry.get("connect") == connect
        and entry.get("request") == request_
        and entry.get("idle") == idle
        for entry in entries
    ), content


@pytest.mark.tier1
@pytest.mark.parametrize(
    "rhc_extra, expected_tmp_dir",
    [
        pytest.param("", "/var/tmp", id="default-tmp-dir"),
        pytest.param(
            '\n[collector]\ntmp-dir = "/var/cache/rhc"\n',
            "/var/cache/rhc",
            id="explicit-tmp-dir",
        ),
    ],
)
def test_collector_tmp_dir_from_configuration(
    rhc, rhc_conf_files, rhc_extra, expected_tmp_dir,
):
    """
    :id: 7a6d2c51-9b4e-4f08-8c7a-1e2d3f4a5b6c
    :title: Verify collector temp directory through rhc startup
    :description:
        rhc-collector creates its private working directories under the
        collector tmp-dir resolved from rhc.conf. Verify the effective tmp-dir,
        both the default and an explicit rhc.conf override.
    :parametrized: yes
    :tags: Tier 1
    :steps:
        1. Back up rhc.conf and rhsm.conf, then write the fixture configuration.
        2. Run rhc with debug logging and an incomplete collector command.
        3. Check the effective collector log entry and restore both files.
    :expectedresults:
        1. The configuration is written at the standard paths.
        2. Startup loads configuration, then exits with usage code 64.
        3. The logged tmp-dir matches; original files are restored.
    """
    rhc_path, rhsm_path = rhc_conf_files
    entries, content = _effective_config_log_entries(
        rhc, rhc_path, rhsm_path, rhc_extra
    )
    assert any(
        entry.get("msg") == "effective collector config"
        and entry.get("tmp-dir") == expected_tmp_dir
        for entry in entries
    ), content
