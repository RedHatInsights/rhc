"""
:casecomponent: rhc
:requirement: RHSS-291300
:subsystemteam: rhel-sst-csi-client-tools
:caseautomation: Automated
:upstream: Yes
"""

import pytest

from utils import yggdrasil_service_is_active


@pytest.mark.tier1
def test_rhc_disconnect(external_candlepin, rhc, test_config):
    """
    :id: 3eb1c32c-fff4-40ae-a659-8b2872d409bf
    :title: Verify that RHC disconnect command disconnects host from server
        and deactivates yggdrasil service
    :description:
        Tests the 'rhc disconnect' command to ensure it unregisters the system
        and stops the associated service.
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
        6.  Stdout contains "Deactivated the yggdrasil service", "Disconnected from
            Red Hat Insights", and "Disconnected from Red Hat Subscription Management".
    """

    # Connect first to perform disconnect operation
    rhc.connect(
        username=test_config.get("candlepin.username"),
        password=test_config.get("candlepin.password"),
    )
    assert rhc.is_registered
    assert yggdrasil_service_is_active()
    disconnect_result = rhc.run("disconnect", check=False)
    assert disconnect_result.returncode == 0
    assert not rhc.is_registered
    assert not yggdrasil_service_is_active()
    assert "Deactivated the yggdrasil service" in disconnect_result.stdout
    assert "Disconnected from Red Hat Insights" in disconnect_result.stdout
    assert (
        "Disconnected from Red Hat Subscription Management" in disconnect_result.stdout
    )


def test_disconnect_when_already_disconnected(rhc):
    """
    :id: 99e6e998-691c-4800-9a81-45c668e6968b
    :title: Test RHC disconnect command when the host is already disconnected
    :description:
        Tests the behavior of the 'rhc disconnect' command when executed on a system
        that is already disconnected.
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
        5.  Stdout contains "Deactivated the yggdrasil service",
            "The yggdrasil service is already inactive",
            "Already disconnected from Red Hat Insights",
            and "Already disconnected from Red Hat Subscription Management".
    """

    # one attempt to disconnect to ensure system is already disconnected
    rhc.run("disconnect", check=False)
    # second attempt to disconnect already disconnected system
    disconnect_result = rhc.run("disconnect", check=False)
    assert disconnect_result.returncode == 0
    assert not rhc.is_registered
    assert "The yggdrasil service is already inactive" in disconnect_result.stdout
    assert "Already disconnected from Red Hat Insights" in disconnect_result.stdout
    assert (
        "Already disconnected from Red Hat Subscription Management"
        in disconnect_result.stdout
    )
