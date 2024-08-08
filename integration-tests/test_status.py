"""
This Python module contains integration tests for rhc. It uses pytest-client-tools Python module.
More information about this module could be found: https://github.com/ptoscano/pytest-client-tools/
"""

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
            "The yggdrasil service is active"
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
    assert "The yggdrasil service is active" in status_result.stdout


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
            "The yggdrasil service is inactive"
    """
    # 'rhc disconnect' to ensure system is already disconnected
    rhc.run("disconnect", check=False)
    status_result = rhc.run("status", check=False)
    assert status_result.returncode == 0
    assert "Not connected to Red Hat Subscription Management" in status_result.stdout
    assert "Not connected to Red Hat Insights" in status_result.stdout
    assert "The yggdrasil service is inactive" in status_result.stdout


def test_yggdrasil_service_restart(external_candlepin, rhc, test_config):
    """
    Test yggdrasil service can be restarted on connected and disconnected system.
    test_steps:
        1- disconnect the system
        2- restart yggdrasil service via systemctl
        3- run rhc connect
        4- restart yggdrasil service via systemctl
    expected_results:
        1- Yggdrasil service should be restarted successfully and set in active state
    """
    # 'rhc disconnect' to ensure system is already disconnected
    rhc.run("disconnect", check=False)
    try:
        util.logged_run("systemctl restart yggdrasil".split())
        assert yggdrasil_service_is_active()
    except AssertionError as exc:
        # for debugging lets check current state of yggdrasil service
        util.logged_run("systemctl status yggdrasil --no-pager".split())
        raise exc

    # test yggdrasil service restart on a connected system
    rhc.connect(
        username=test_config.get("candlepin.username"),
        password=test_config.get("candlepin.password"),
    )
    assert rhc.is_registered
    try:
        util.logged_run("systemctl restart yggdrasil".split())
        assert yggdrasil_service_is_active()
    except AssertionError as exc:
        util.logged_run("systemctl status yggdrasil --no-pager".split())
        raise exc
