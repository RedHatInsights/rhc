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
