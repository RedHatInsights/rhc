"""
This Python module contains integration tests for rhc.
It uses pytest-client-tools Python module. More information about
this module could be found: https://github.com/RedHatInsights/pytest-client-tools/
"""

import pytest
import re

from utils import rhcd_service_is_active


def test_rhc_disconnect(external_candlepin, rhc, test_config):
    """Verify that RHC disconnect command disconnects host from server
    and deactivates rhcd service.
    test_steps:
        1- run rhc connect
        2- run rhc disconnect
    expected_output:
        1- Assert exit code 0
        2- Validate that these string are present in the stdout:
            "Deactivated the Remote Host Configuration daemon",
            "Disconnected from Red Hat Insights",
            "Disconnected from Red Hat Subscription Management"
    """
    # Connect first to perform disconnect operation
    rhc.connect(
        activationkey=test_config.get("candlepin.activation_keys")[0],
        org=test_config.get("candlepin.org"),
    )
    assert rhc.is_registered
    assert rhcd_service_is_active()
    disconnect_result = rhc.run("disconnect", check=False)
    assert disconnect_result.returncode == 0
    assert not rhc.is_registered
    assert not rhcd_service_is_active()
    # copr builds have BrandName rhc and downstream builds have "Remote Host Configuration"
    assert re.search(
        r"(Deactivated the Remote Host Configuration daemon|Deactivated the rhc daemon)",
        disconnect_result.stdout,
    )
    assert "Disconnected from Red Hat Insights" in disconnect_result.stdout
    assert (
        "Disconnected from Red Hat Subscription Management" in disconnect_result.stdout
    )


# @pytest.mark.skip(
#   reason="Test cannot be run due to unresolved issue https://issues.redhat.com/browse/CCT-525"
# )
def test_disconnect_when_already_disconnected(rhc):
    """Test RHC disconnect command when the host is already
    disconnected from CRC
    test_steps:
      # rhc disconnect
    expected_output:
      1. validate that these string are present in the stdout:
            "Deactivated the rhcd service",
            "Cannot disconnect from Red Hat Subscription Management",
            "insights  cannot disconnect from Red Hat Insights",
            "rhsm      cannot disconnect from Red Hat Subscription Management: "
            "warning: the system is already unregistered",
    """
    # one attempt to disconnect to ensure system is already disconnected
    rhc.run("disconnect", check=False)
    # second attempt to disconnect already disconnected system
    disconnect_result = rhc.run("disconnect", check=False)
    assert disconnect_result.returncode == 1
    assert not rhc.is_registered
    assert re.search(
        r"(Deactivated the Remote Host Configuration daemon|Deactivated the rhc daemon)",
        disconnect_result.stdout,
    )
    assert "Cannot disconnect from Red Hat Insights" in disconnect_result.stdout
    assert (
        "Cannot disconnect from Red Hat Subscription Management"
        in disconnect_result.stdout
    )
    assert (
        "rhsm      cannot disconnect from Red Hat Subscription Management:"
        in disconnect_result.stdout
    )
    assert "warning: the system is already unregistered" in disconnect_result.stdout
    assert (
        "ERROR  insights  cannot disconnect from Red Hat Insights"
        in disconnect_result.stdout
    )
