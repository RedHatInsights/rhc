"""
This Python module contains integration tests for rhc. It uses pytest-client-tools Python module.
More information about this module could be found: https://github.com/RedHatInsights/pytest-client-tools/
"""

"""
:component: rhc
:requirement: RHSS-291300
:polarion-project-id: RHELSS
:polarion-include-skipped: false
:polarion-lookup-method: id
:poolteam: rhel-sst-csi-client-tools
:caseautomation: Automated
:upstream: Yes
"""

import pytest
import json
from pytest_client_tools import util
from utils import yggdrasil_service_is_active


@pytest.mark.tier1
def test_status_connected(external_candlepin, rhc, test_config):
    """
    :id: b352465d-d1ae-424b-b741-cef7451a2a18
    :title: Verify RHC status command output when the host is connected
    :description:
        Test the output of the 'rhc status' command when the host is connected
        to Subscription Management and Red Hat Insights.
    :tags: Tier 1
    :steps:
        1.  Connect the system using 'rhc connect'.
        2.  Ensure the yggdrasil/rhcd service is active.
        3.  Run the 'rhc status' command.
        4.  Verify the command exit code.
        5.  Verify specific strings are present in the standard output
            indicating connectivity and yggdrasil/rhcd service status.
    :expectedresults:
        1.  RHC connects successfully.
        2.  The yggdrasil/rhcd service is active.
        3.  The 'rhc status' command executes successfully.
        4.  The exit code is 0.
        5.  The output contains "Connected to Red Hat Subscription Management",
            "Connected to Red Hat Insights", "Red Hat repository file generated",
            and "The yggdrasil service is active" or "The Remote Host Configuration daemon is active".
    """

    rhc.connect(
        username=test_config.get("candlepin.username"),
        password=test_config.get("candlepin.password"),
    )
    assert yggdrasil_service_is_active()
    status_result = rhc.run("status", check=False)
    assert status_result.returncode == 0
    assert "Connected to Red Hat Subscription Management" in status_result.stdout
    assert "Red Hat repository file generated" in status_result.stdout
    assert "Connected to Red Hat Insights" in status_result.stdout
    if pytest.service_name == "rhcd":
        assert "The Remote Host Configuration daemon is active" in status_result.stdout
    else:
        assert "The yggdrasil service is active" in status_result.stdout


def test_status_connected_format_json(external_candlepin, rhc, test_config):
    """
    :id: 6807fc50-156c-41a0-bc58-8f408e417a70
    :title: Verify RHC status command output in JSON format when the host is connected
    :description:
        Test the output of the 'rhc status --format json' command when the host is
        connected to Subscription Management and Insights, verifying the output
        is valid JSON and contains expected data.
    :tags:
    :steps:
        1.  Connect the system using 'rhc connect'.
        2.  Run the 'rhc status --format json' command.
        3.  Check the exit code of the status command.
        4.  Parse the JSON output.
        5.  Verify the presence and data types of specific keys.
    :expectedresults:
        1.  RHC connects successfully.
        2.  The 'rhc status --format json' command executes successfully.
        3.  The exit code is 0.
        4.  The output is a valid JSON document.
        5.  The JSON contains 'hostname', 'rhsm_connected' (true), 'content_enabled' (true),
            'insights_connected' (true), and 'rhcd_running' or 'yggdrasil_running' (true)
            with correct boolean types.
    """

    rhc.connect(
        username=test_config.get("candlepin.username"),
        password=test_config.get("candlepin.password"),
    )
    status_result = rhc.run("status", "--format", "json", check=False)
    assert status_result.returncode == 0
    status_json = json.loads(status_result.stdout)
    assert "hostname" in status_json
    assert "rhsm_connected" in status_json
    assert type(status_json["rhsm_connected"]) == bool
    assert status_json["rhsm_connected"] == True
    assert "content_enabled" in status_json
    assert type(status_json["content_enabled"]) == bool
    assert status_json["content_enabled"] == True
    assert "insights_connected" in status_json
    assert type(status_json["insights_connected"]) == bool
    assert status_json["insights_connected"] == True
    if pytest.service_name == "rhcd":
        assert "rhcd_running" in status_json
        assert type(status_json["rhcd_running"]) == bool
        assert status_json["rhcd_running"] == True
    else:
        assert "yggdrasil_running" in status_json
        assert type(status_json["yggdrasil_running"]) == bool
        assert status_json["yggdrasil_running"] == True


def test_status_disconnected_format_json(external_candlepin, rhc, test_config):
    """
    :id: 4ba5fcb5-3cc3-456c-8873-f03abd7c9451
    :title: Verify RHC status command output in JSON format when the host is disconnected
    :description:
        This test verifies the output of the 'rhc status --format json' command
        when the host is disconnected, ensuring the output is valid JSON
        and reflects the disconnected state.
    :tags:
    :steps:
        1.  Disconnect the system using 'rhc disconnect'.
        2.  Run the 'rhc status --format json' command.
        3.  Check the exit code of the command.
        4.  Parse the JSON output.
        5.  Verify the presence and data types of specific keys.
    :expectedresults:
        1.  RHC disconnects successfully.
        2.  The 'rhc status --format json' command executes successfully.
        3.  The exit code is 0.
        4.  The output is a valid JSON document.
        5.  The JSON contains 'hostname', 'rhsm_connected' (false), 'content_enabled' (false),
            'insights_connected' (false), and 'rhcd_running' or 'yggdrasil_running' (false)
            with correct boolean types.
    """

    rhc.run("disconnect", check=False)
    status_result = rhc.run("status", "--format", "json", check=False)
    assert status_result.returncode != 0
    status_json = json.loads(status_result.stdout)
    assert "hostname" in status_json
    assert "rhsm_connected" in status_json
    assert type(status_json["rhsm_connected"]) == bool
    assert status_json["rhsm_connected"] == False
    assert "content_enabled" in status_json
    assert type(status_json["content_enabled"]) == bool
    assert status_json["content_enabled"] == False
    assert "insights_connected" in status_json
    assert type(status_json["insights_connected"]) == bool
    assert status_json["insights_connected"] == False
    if pytest.service_name == "rhcd":
        assert "rhcd_running" in status_json
        assert type(status_json["rhcd_running"]) == bool
        assert status_json["rhcd_running"] == False
    else:
        assert "yggdrasil_running" in status_json
        assert type(status_json["yggdrasil_running"]) == bool
        assert status_json["yggdrasil_running"] == False


@pytest.mark.tier1
def test_status_disconnected(rhc):
    """
    :id: b2587673-4d9e-4b18-bebd-dfe4b8874622
    :title: Verify RHC status command output when the host is disconnected
    :description:
        This test verifies the output of the 'rhc status' command when the host
        is disconnected from Red Hat Subscription Management and Red Hat Insights,
        and the yggdrasil/rhcd service is inactive.
    :reference: https://issues.redhat.com/browse/CCT-525
    :tags: Tier 1
    :steps:
        1.  Disconnect the system using 'rhc disconnect'.
        2.  Run the 'rhc status' command.
        3.  Verify the command exit code.
        4.  Verify specific output strings in stdout.
    :expectedresults:
        1.  RHC disconnects successfully.
        2.  The command executes.
        3.  The exit code is 0.
        4.  The status command output contains "Not connected to Red Hat Subscription Management",
            "Red Hat repository file not generated", "Not connected to Red Hat Insights",
            and a message indicating that the yggdrasil/rhcd service is inactive.
    """

    # 'rhc disconnect' to ensure system is already disconnected
    rhc.run("disconnect", check=False)
    status_result = rhc.run("status", check=False)
    assert status_result.returncode != 0
    assert "Not connected to Red Hat Subscription Management" in status_result.stdout
    assert "Red Hat repository file not generated" in status_result.stdout
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
    :id: 92dbb5e7-c16c-4f5f-9c33-84e9e1269dde
    :title: Verify rhcd service restart functionality
    :description:
        This test verifies that the rhcd service can be successfully restarted
        when the system is connected and that its status is correctly reflected
        after restart. This test is specifically for the 'rhcd' service and skips
        if the service name is not 'rhcd'.
    :tags:
    :steps:
        1. Disconnect the system using 'rhc disconnect' to ensure a clean state.
        2. Restart the rhcd service using 'systemctl restart rhcd'.
        3. Connect the system using 'rhc connect'.
        4. Restart the rhcd service again using 'systemctl restart rhcd'.
    :expectedresults:
        1. RHC disconnects successfully.
        2. The rhcd service restarts successfully, and it's inactive.
        3. The system connects successfully and is registered.
        4. The rhcd service restarts successfully, and it's active.
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
