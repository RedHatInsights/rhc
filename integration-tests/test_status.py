"""
This Python module contains integration tests for rhc. It uses pytest-client-tools Python module.
More information about this module could be found: https://github.com/ptoscano/pytest-client-tools/
"""

import pytest
import json
from pytest_client_tools import util
from utils import yggdrasil_service_is_active


def test_status_connected(external_candlepin, rhc, test_config):
    """Test RHC Status command when the host is connected.
    test_steps:
        1- rhc connect
        2- rhc status
    expected_output:
        1- Validate following strings in status command output
            "Connected to Red Hat Subscription Management"
            "Connected to Red Hat Insights"
            "The yggdrasil/rhcd service is active"
    """
    rhc.connect(
        username=test_config.get("candlepin.username"),
        password=test_config.get("candlepin.password"),
    )
    assert yggdrasil_service_is_active()
    status_result = rhc.run("status", check=False)
    assert status_result.returncode == 0
    assert "Connected to Red Hat Subscription Management" in status_result.stdout
    assert "Connected to Red Hat Insights" in status_result.stdout
    if pytest.service_name == "rhcd":
        assert "The Remote Host Configuration daemon is active" in status_result.stdout
    else:
        assert "The yggdrasil service is active" in status_result.stdout


def test_status_connected_format_json(external_candlepin, rhc, test_config):
    """
    Test 'rhc status --format json' command, when host is connected
    test_steps:
        1 - rhc connect
        2 - rhc status
    expected_output:
        1 - Validate that output is valid JSON document
        2 - Validate that JSON document contains expected data
    """
    rhc.connect(
        username=test_config.get("candlepin.username"),
        password=test_config.get("candlepin.password")
    )
    status_result = rhc.run("status", "--format", "json", check=False)
    assert status_result.returncode == 0
    status_json = json.loads(status_result.stdout)
    assert "hostname" in status_json
    assert "rhsm_connected" in status_json
    assert type(status_json["rhsm_connected"]) == bool
    assert "insights_connected" in status_json
    assert type(status_json["insights_connected"]) == bool
    if pytest.service_name == "rhcd":
        assert "rhcd_running" in status_json
        assert type(status_json["rhcd_running"]) == bool
    else:
        assert "yggdrasil_running" in status_json
        assert type(status_json["yggdrasil_running"]) == bool


def test_status_disconnected(rhc):
    """Test RHC Status command when the host is disconnected.
    Ref: https://issues.redhat.com/browse/CCT-525
    test_steps:
        1- rhc disconnect
        2- rhc status
    expected_output:
        1- Validate following strings in status command output
            "Not connected to Red Hat Subscription Management"
            "Not connected to Red Hat Insights"
            "The yggdrasil/rhc service is inactive"
    """
    # 'rhc disconnect' to ensure system is already disconnected
    rhc.run("disconnect", check=False)
    status_result = rhc.run("status", check=False)
    assert status_result.returncode != 0
    assert "Not connected to Red Hat Subscription Management" in status_result.stdout
    assert "Not connected to Red Hat Insights" in status_result.stdout
    if pytest.service_name == "rhcd":
        assert "The Remote Host Configuration daemon is active" in status_result.stdout
    else:
        assert "The yggdrasil service is inactive" in status_result.stdout


@pytest.mark.skipif(
    pytest.service_name != "rhcd",
    reason="This test only supports restart of rhcd and not yggdrasil",
)
def test_rhcd_service_restart(external_candlepin, rhc, test_config):
    """
    Test rhcd service can be restarted on connected and  not on disconnected system.
    test_steps:
        1- disconnect the system
        2- restart rhcd service via systemctl
        3- run rhc connect
        4- restart rhcd service via systemctl
    expected_results:
        1- rhcd  should be restarted successfully on connected system
    """

    # 'rhc disconnect' to ensure system is already disconnected
    rhc.run("disconnect", check=False)
    util.logged_run("systemctl status rhcd --no-pager".split())
    try:

        util.logged_run("systemctl restart rhcd".split())
        assert not yggdrasil_service_is_active()
    except AssertionError as exc:
        # for debugging lets check current state of rhcd service
        util.logged_run("systemctl status rhcd --no-pager".split())
        raise exc

    # test rhcd service restart on a connected system
    rhc.connect(
        username=test_config.get("candlepin.username"),
        password=test_config.get("candlepin.password"),
    )
    assert rhc.is_registered
    try:
        util.logged_run("systemctl restart rhcd".split())
        assert yggdrasil_service_is_active()
    except AssertionError as exc:
        util.logged_run("systemctl status rhcd --no-pager".split())
        raise exc
