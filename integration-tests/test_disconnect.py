"""
:casecomponent: rhc
:requirement: RHSS-291300
:subsystemteam: rhel-sst-csi-client-tools
:caseautomation: Automated
:upstream: Yes
"""

import json
import pytest

from utils import yggdrasil_service_is_active


@pytest.mark.tier1
@pytest.mark.parametrize(
    "output_format",[None, "json"],
    ids=["output-format=None", "output-format=json"],
)
def test_rhc_disconnect(external_candlepin, rhc, test_config, output_format):
    """
    :id: 3eb1c32c-fff4-40ae-a659-8b2872d409bf
    :title: Verify that RHC disconnect command disconnects host from server
        and deactivates yggdrasil service
    :description:
        Tests the 'rhc disconnect' command to ensure it unregisters the system
        and stops the associated service. Supports both text and JSON output formats.
    :tags: Tier 1
    :steps:
        1.  Connect the system using rhc connect.
        2.  Verify the system is registered and the yggdrasil service is active.
        3.  Run the rhc disconnect command.
        4.  Verify the command exit code.
        5.  Verify the system is no longer registered and the yggdrasil service is inactive.
        6.  Verify specific output strings in stdout.
    :expectedresults:
        1.  System connects successfully and is registered.
        2.  The system is registered and the yggdrasil service is active.
        3.  The command executes.
        4.  The exit code is 0.
        5.  The system is unregistered and the yggdrasil service is inactive.
        6.  For text output, stdout contains "Deactivated the yggdrasil service",
            "Disconnected from Red Hat Lightspeed (formerly Insights)", and "Disconnected from Red Hat Subscription Management".
            For JSON output, comprehensive validation is performed on the response values.
    """

    # Connect first to perform disconnect operation
    rhc.connect(
        activationkey=test_config.get("candlepin.activation_keys")[0],
        org=test_config.get("candlepin.org"),
    )
    assert rhc.is_registered
    assert yggdrasil_service_is_active()
    if output_format == "json":
        disconnect_result = rhc.run("disconnect", "--format", "json", check=False)
    else:
        disconnect_result = rhc.run("disconnect", check=False)
    assert disconnect_result.returncode == 0
    assert not rhc.is_registered
    assert not yggdrasil_service_is_active()

    if output_format is None:
        # plain text checks
        assert "Deactivated the yggdrasil service" in disconnect_result.stdout
        assert "Disconnected from Red Hat Lightspeed (formerly Insights)" in disconnect_result.stdout
        assert (
            "Disconnected from Red Hat Subscription Management"
            in disconnect_result.stdout
        )
    elif output_format == "json":
        # JSON checks
        json_output = json.loads(disconnect_result.stdout)
        assert json_output["rhsm_disconnected"] is True
        assert json_output["insights_disconnected"] is True
        assert json_output["yggdrasil_stopped"] is True


@pytest.mark.parametrize(
    "output_format",
    [None, "json"],
    ids=["output-format=None", "output-format=json"],
)
def test_disconnect_when_already_disconnected(rhc, output_format):
    """
    :id: 99e6e998-691c-4800-9a81-45c668e6968b
    :title: Test RHC disconnect command when the host is already disconnected
    :description:
        Tests the behavior of the 'rhc disconnect' command when executed on a system
        that is already disconnected. Supports both text and JSON output formats.
    :tags: Tier 1
    :steps:
        1.  Ensure the system is disconnected by running rhc disconnect once (allowing failure).
        2.  Run the 'rhc disconnect' command again on the already disconnected system.
        3.  Verify the command exit code.
        4.  Verify the system remains unregistered.
        5.  Verify specific error/warning messages are present in stdout/stderr.
    :expectedresults:
        1.  The disconnect command attempts to run.
        2.  The command executes.
        3.  The exit code 0
        4.  The system is unregistered.
        5.  For text output, stdout contains "The yggdrasil service is already inactive",
            "Already disconnected from Red Hat Lightspeed (formerly Insights)",
            and "Already disconnected from Red Hat Subscription Management".
            For JSON output, comprehensive validation is performed on the response structure and values.
    """

    # one attempt to disconnect to ensure system is already disconnected
    rhc.run("disconnect", check=False)
    # second attempt to disconnect already disconnected system
    if output_format == "json":
        disconnect_result = rhc.run("disconnect", "--format", "json", check=False)
    else:
        disconnect_result = rhc.run("disconnect", check=False)
    assert disconnect_result.returncode == 0
    assert not rhc.is_registered

    if output_format is None:
        # plain text checks
        assert "The yggdrasil service is already inactive" in disconnect_result.stdout
        assert "Already disconnected from Red Hat Lightspeed (formerly Insights)" in disconnect_result.stdout
        assert (
            "Already disconnected from Red Hat Subscription Management"
            in disconnect_result.stdout
        )
    elif output_format == "json":
        # JSON checks
        json_output = json.loads(disconnect_result.stdout)
        assert json_output["rhsm_disconnected"] is True
        assert json_output["insights_disconnected"] is True
        assert json_output["yggdrasil_stopped"] is True
