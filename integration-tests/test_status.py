"""
:casecomponent: rhc
:requirement: RHSS-291300
:subsystemteam: rhel-sst-csi-client-tools
:caseautomation: Automated
:upstream: Yes
"""

import pytest
import json
import subprocess
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
        2.  Ensure the yggdrasil service is active.
        3.  Run the 'rhc status' command.
        4.  Verify the command exit code.
        5.  Verify specific strings are present in the standard output
            indicating connectivity and yggdrasil service status.
    :expectedresults:
        1.  RHC connects successfully.
        2.  The yggdrasil service is active.
        3.  The 'rhc status' command executes successfully.
        4.  The exit code is 0.
        5.  The output contains "Connected to Red Hat Subscription Management",
            "Connected to Red Hat Insights", "Red Hat repository file generated",
            and "The yggdrasil service is active".
    """

    rhc.connect(
        activationkey=test_config.get("candlepin.activation_keys")[0],
        org=test_config.get("candlepin.org"),
    )
    assert yggdrasil_service_is_active()
    status_result = rhc.run("status", check=False)
    assert status_result.returncode == 0
    assert "Connected to Red Hat Subscription Management" in status_result.stdout
    assert "Red Hat repository file generated" in status_result.stdout
    assert "Connected to Red Hat Insights" in status_result.stdout
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
            'insights_connected' (true), and 'yggdrasil_running' (true)
            with correct boolean types.
    """

    rhc.connect(
        activationkey=test_config.get("candlepin.activation_keys")[0],
        org=test_config.get("candlepin.org"),
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
        3.  The exit code is not 0.
        4.  The output is a valid JSON document.
        5.  The JSON contains 'hostname', 'rhsm_connected' (false), 'content_enabled' (false),
            'insights_connected' (false), and 'yggdrasil_running' (false)
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
        and the yggdrasil service is inactive.
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
        3.  The exit code is not 0.
        4.  The status command output contains "Not connected to Red Hat Subscription Management",
            "Red Hat repository file not generated", "Not connected to Red Hat Insights",
            and a message indicating that the yggdrasil service is inactive.
    """

    # 'rhc disconnect' to ensure system is already disconnected
    rhc.run("disconnect", check=False)
    status_result = rhc.run("status", check=False)
    assert status_result.returncode != 0
    assert "Not connected to Red Hat Subscription Management" in status_result.stdout
    assert "Red Hat repository file not generated" in status_result.stdout
    assert "Not connected to Red Hat Insights" in status_result.stdout
    assert "The yggdrasil service is inactive" in status_result.stdout


@pytest.fixture
def unmask_rhsm_service():
    """
    pytest fixture to unmask rhsm.service
    """
    yield
    subprocess.run(["systemctl", "unmask", "rhsm.service"])


@pytest.mark.tier1
@pytest.mark.usefixtures("unmask_rhsm_service")
def test_status_connected_rhsm_masked(external_candlepin, rhc, test_config):
    """
    :id: 81e1ef2a-cc29-4f9c-9780-cfd027aa08ec
    :title: Verify RHC status command output when the host is connected and rhsm service is masked
    :description:
        This test verifies the output of the 'rhc status' command when the rhsm.service
        is masked and calling RHSM D-Bus API is not possible. We test this case for
        a connected system. It means that other features should be active.
    :reference: https://issues.redhat.com/browse/CCT-1526
    :tags: Tier 1
    :steps:
        1.  Connect the system using 'rhc connect'.
        2.  Stop and mask the 'rhsm.service'.
        3.  Run the 'rhc status' command.
        4.  Verify the command exit code.
        5.  Verify specific output strings in stdout.
        6.  Unmask 'rhsm.service'.
    :expectedresults:
        1.  RHC connects successfully.
        2.  The 'rhsm.service' is stopped and masked.
        3.  The command executes and fails.
        4.  The exit code is not 0 because there were errors.
        5.  The status command output contains "Could not activate remote peer",
            "Connected to Red Hat Insights", and a message indicating
            that the yggdrasil service is active.
        6.  The 'rhsm.service' is unmasked.
    """

    rhc.connect(
        activationkey=test_config.get("candlepin.activation_keys")[0],
        org=test_config.get("candlepin.org"),
    )

    # stop and mask rhsm.service
    subprocess.run(["systemctl", "stop", "rhsm.service"])
    subprocess.run(["systemctl", "mask", "rhsm.service"])

    # Run "rhc status" and check the result
    status_result = rhc.run("status", check=False)
    assert status_result.returncode != 0
    assert "Could not activate remote peer" in status_result.stdout

    # Test other features
    assert "Connected to Red Hat Insights" in status_result.stdout
    assert yggdrasil_service_is_active()
    assert "The yggdrasil service is active" in status_result.stdout


@pytest.mark.tier1
@pytest.mark.usefixtures("unmask_rhsm_service")
def test_status_disconnected_rhsm_masked(rhc):
    """
    :id: 704a91e5-d509-4635-a401-82368c7bdd75
    :title: Verify RHC status command output when the host is disconnected and rhsm service is masked
    :description:
        This test verifies the output of the 'rhc status' command when the rhsm.service
        is masked and calling RHSM D-Bus API is not possible. We test this case for
        a disconnected system. It means that other features are inactive.
    :reference: https://issues.redhat.com/browse/CCT-1526
    :tags: Tier 1
    :steps:
        1.  Disconnect the system using 'rhc disconnect'.
        2.  Stop and mask the 'rhsm.service'.
        3.  Run the 'rhc status' command.
        4.  Verify the command exit code.
        5.  Verify specific output strings in stdout.
        6.  Unmask 'rhsm.service'.
    :expectedresults:
        1.  RHC disconnects successfully.
        2.  The 'rhsm.service' is stopped and masked.
        3.  The command executes and fails.
        4.  The exit code is not 0.
        5.  The status command output contains "Could not activate remote peer",
            "Not connected to Red Hat Insights", and a message indicating
            that the yggdrasil service is inactive.
        6.  The 'rhsm.service' is unmasked.
    """

    # 'rhc disconnect' to ensure the system is already disconnected
    rhc.run("disconnect", check=False)
    # stop and mask rhsm.service
    subprocess.run(["systemctl", "stop", "rhsm.service"])
    subprocess.run(["systemctl", "mask", "rhsm.service"])
    status_result = rhc.run("status", check=False)
    assert status_result.returncode != 0
    assert "Could not activate remote peer" in status_result.stdout
    assert "Not connected to Red Hat Insights" in status_result.stdout
    assert "The yggdrasil service is inactive" in status_result.stdout


@pytest.mark.tier1
@pytest.mark.usefixtures("unmask_rhsm_service")
def test_status_disconnected_rhsm_masked_format_json(rhc):
    """
    :id: 0ec86207-827f-4839-a49d-041c0525c1a8
    :title: Verify RHC status command output in JSON format when the host is disconnected and rhsm service is masked
    :description:
        This test verifies the output of the 'rhc status --format json' command when the rhsm.service
        is masked and calling RHSM D-Bus API is not possible. We test this case for
        a disconnected system. It means that other features are inactive.
    :reference: https://issues.redhat.com/browse/CCT-1526
    :tags: Tier 1
    :steps:
        1.  Disconnect the system using 'rhc disconnect'.
        2.  Stop and mask the 'rhsm.service'.
        3.  Run the 'rhc status --format json' command.
        4.  Verify the command exit code.
        5.  Parse the JSON output.
        6.  Verify the boolean values and error messages in the JSON fields.
        7.  Unmask 'rhsm.service'.
    :expectedresults:
        1.  RHC disconnects successfully.
        2.  The 'rhsm.service' is stopped and masked.
        3.  The command executes and fails.
        4.  The exit code is not 0.
        5.  The output is a valid JSON document.
        6.  The JSON contains 'hostname', 'rhsm_connected' (false), 'content_enabled' (false),
            'insights_connected' (false), and 'yggdrasil_running' (false)
            with correct boolean types. It also contains 'rhsm_error' and 'content_error' messages.
        7.  The 'rhsm.service' is unmasked.
    """

    # 'rhc disconnect' to ensure the system is already disconnected
    rhc.run("disconnect", check=False)
    # stop and mask rhsm.service
    subprocess.run(["systemctl", "stop", "rhsm.service"])
    subprocess.run(["systemctl", "mask", "rhsm.service"])

    # Run "rhc status --format json" and check the result
    status_result = rhc.run("status", "--format", "json", check=False)
    assert status_result.returncode != 0

    status_json = json.loads(status_result.stdout)
    assert "hostname" in status_json

    # rhsm
    assert "rhsm_connected" in status_json
    assert type(status_json["rhsm_connected"]) == bool
    assert status_json["rhsm_connected"] == False
    assert "rhsm_error" in status_json
    assert type(status_json["rhsm_error"]) == str
    assert "Could not activate remote peer" in status_json["rhsm_error"]

    # content
    assert "content_enabled" in status_json
    assert type(status_json["content_enabled"]) == bool
    assert status_json["content_enabled"] == False
    assert "content_error" in status_json
    assert type(status_json["content_error"]) == str
    assert "Could not activate remote peer" in status_json["content_error"]

    # insights
    assert "insights_connected" in status_json
    assert type(status_json["insights_connected"]) == bool
    assert status_json["insights_connected"] == False

    # yggdrasil
    assert "yggdrasil_running" in status_json
    assert type(status_json["yggdrasil_running"]) == bool
    assert status_json["yggdrasil_running"] == False


@pytest.fixture
def unmask_yggdrasil_service():
    """
    pytest fixture to unmask rhsm.service
    """
    yield
    subprocess.run(["systemctl", "unmask", "yggdrasil.service"])


@pytest.mark.tier1
@pytest.mark.usefixtures("unmask_yggdrasil_service")
def test_status_connected_yggdrasil_masked(external_candlepin, rhc, test_config):
    """
    :id: 316993d3-cc8d-4a10-9166-2117fa43396a
    :title: Verify RHC status command output when the host is connected and yggdrasil service is masked
    :description:
        This test verifies the output of the 'rhc status' command when the yggdrasil.service
        is masked and getting status is not possible. We test this case for
        a connected system.
    :reference: https://issues.redhat.com/browse/CCT-1526
    :tags: Tier 1
    :steps:
        1.  Connect the system using 'rhc connect'.
        2.  Stop and mask the 'yggdrasil.service'.
        3.  Run the 'rhc status' command.
        4.  Verify the command exit code.
        5.  Verify specific output strings in stdout.
        6.  Unmask 'yggdrasil.service'.
    :expectedresults:
        1.  RHC connects successfully.
        2.  The 'yggdrasil.service' is stopped and masked.
        3.  The command executes and fails.
        4.  The exit code is not 0.
        5.  The output contains "Connected to Red Hat Subscription Management",
            "Connected to Red Hat Insights", "Red Hat repository file generated",
            "Unit yggdrasil.service is masked"
        6.  The 'yggdrasil.service' is unmasked.
    """

    rhc.connect(
        activationkey=test_config.get("candlepin.activation_keys")[0],
        org=test_config.get("candlepin.org"),
    )

    # stop and mask yggdrasil.service
    subprocess.run(["systemctl", "stop", "yggdrasil.service"])
    subprocess.run(["systemctl", "mask", "yggdrasil.service"])

    # Run "rhc status" and check the result
    status_result = rhc.run("status", check=False)
    assert status_result.returncode != 0
    # RHSM
    assert "Connected to Red Hat Subscription Management" in status_result.stdout
    assert "Red Hat repository file generated" in status_result.stdout
    # Insights
    assert "Connected to Red Hat Insights" in status_result.stdout
    # yggdrasil
    assert not yggdrasil_service_is_active()
    assert "Unit yggdrasil.service is masked" in status_result.stdout


@pytest.mark.tier1
@pytest.mark.usefixtures("unmask_yggdrasil_service")
def test_status_connected_yggdrasil_masked_format_json(external_candlepin, rhc, test_config):
    """
    :id: ff0d0da8-6396-4f3a-80e6-dce27157bc30
    :title: Verify RHC status command output in JSON format when the host is connected
        and yggdrasil service is masked
    :description:
        Test the output of the 'rhc status --format json' command when the host is
        connected to Subscription Management and Insights, but the yggdrasil.service
        was masked. Verify that the command exit code is not zero and the output
        is valid JSON and contains expected data.
    :tags:
    :steps:
        1.  Connect the system using 'rhc connect'.
        2.  Stop and mask the 'yggdrasil.service'.
        3.  Run the 'rhc status --format json' command.
        4.  Verify the command exit code.
        5.  Parse the JSON output.
        6.  Verify the presence and data types of specific keys.
        7.  Unmask 'yggdrasil.service'.
    :expectedresults:
        1.  RHC connects successfully.
        2.  The 'yggdrasil.service' is stopped and masked.
        3.  The command executes and fails.
        4.  The exit code is not 0.
        5.  The output is a valid JSON document.
        6.  The JSON contains 'hostname', 'rhsm_connected' (true), 'content_enabled' (true),
            'insights_connected' (true), 'yggdrasil_running' (false)
            and yggdrasil_error with the expected error message.
        7.  The 'yggdrasil.service' is unmasked.
    """

    rhc.connect(
        activationkey=test_config.get("candlepin.activation_keys")[0],
        org=test_config.get("candlepin.org"),
    )
    # stop and mask yggdrasil.service
    subprocess.run(["systemctl", "stop", "yggdrasil.service"])
    subprocess.run(["systemctl", "mask", "yggdrasil.service"])
    # Run "rhc status --format json" and check the result
    status_result = rhc.run("status", "--format", "json", check=False)
    assert status_result.returncode != 0
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
    assert "yggdrasil_running" in status_json
    assert type(status_json["yggdrasil_running"]) == bool
    assert status_json["yggdrasil_running"] == False
    assert "yggdrasil_error" in status_json
    assert type(status_json["yggdrasil_error"]) == str
    assert "Unit yggdrasil.service is masked" in status_json["yggdrasil_error"]


def test_yggdrasil_service_restart(external_candlepin, rhc, test_config):
    """
    :id: 92dbb5e7-c16c-4f5f-9c33-84e9e1269dde
    :title: Verify yggdrasil service restart functionality
    :description:
        This test verifies that the yggdrasil service can be successfully restarted
        when the system is connected and that its status is correctly reflected
        after restart.
    :tags:
    :steps:
        1. Disconnect the system using 'rhc disconnect' to ensure a clean state.
        2. Restart the yggdrasil service using 'systemctl restart yggdrasil'.
        3. Connect the system using 'rhc connect'.
        4. Restart the yggdrasil service again using 'systemctl restart yggdrasil'.
    :expectedresults:
        1. RHC disconnects successfully.
        2. The yggdrasil service restarts successfully, and it's inactive.
        3. The system connects successfully and is registered.
        4. The yggdrasil service restarts successfully, and it's active.
    """

    # 'rhc disconnect' to ensure system is already disconnected
    rhc.run("disconnect", check=False)
    util.logged_run("systemctl status yggdrasil --no-pager".split())
    try:

        util.logged_run("systemctl restart yggdrasil".split())
        assert not yggdrasil_service_is_active()
    except AssertionError as exc:
        # for debugging lets check current state of yggdrasil service
        util.logged_run("systemctl status yggdrasil --no-pager".split())
        raise exc

    # test yggdrasil service restart on a connected system
    rhc.connect(
        activationkey=test_config.get("candlepin.activation_keys")[0],
        org=test_config.get("candlepin.org"),
    )
    assert rhc.is_registered
    try:
        util.logged_run("systemctl restart yggdrasil".split())
        assert yggdrasil_service_is_active()
    except AssertionError as exc:
        util.logged_run("systemctl status yggdrasil --no-pager".split())
        raise exc
