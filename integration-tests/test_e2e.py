"""
This Python module contains integration tests for rhc.
It uses pytest-client-tools Python module.More information about this
module could be found: https://github.com/RedHatInsights/pytest-client-tools/
"""

"""
:casecomponent: rhc
:requirement: RHSS-291300
:polarion-project-id: RHELSS
:polarion-include-skipped: false
:polarion-lookup-method: id
:subsystemteam: rhel-sst-csi-client-tools
:caseautomation: Automated
:upstream: Yes
"""

import contextlib
import pytest
import time
from utils import (
    yggdrasil_service_is_active,
    prepare_args_for_connect,
)


@pytest.mark.skip("Test cannot be run due to unresolved issues CCT-862 and CCT-696")
@pytest.mark.parametrize("auth", ["activation-key", "basic"])
def test_rhc_client_connect_e2e(rhc, test_config, auth, external_inventory, subman):
    """
    :id: bbac96bd-a551-423c-b00b-1e62a743d4ed
    :title: Test RHC client connect end-to-end with basic auth and activation key
    :parametrized: yes
    :description:
        This test verifies the end-to-end functionality of the RHC client connect command
        using both basic authentication and an activation key. It checks if the client
        registers successfully, the yggdrasil/rhcd service starts, the host appears in Inventory,
        and the client ID in the system profile matches the subscription manager UUID.
    :tags:
    :steps:
        1.  Attempt to disconnect the RHC client to ensure a clean state.
        2.  Run the 'rhc connect' command with the prepared arguments.
        3.  Verify that the RHC client reports a registered status.
        4.  Verify that the yggdrasil/rhcd service is running and active.
        5.  Check if the system profile contains the 'rhc_client_id'.
        6.  Verify that the 'rhc_client_id' in the system profile matches the subscription manager UUID.
    :expectedresults:
        1.  The client is disconnected, or the action is suppressed if no connection exists.
        2.  The 'rhc connect' command executes successfully.
        3.  The RHC client status indicates registration.
        4.  The yggdrasil/rhcd service is active.
        5.  The 'rhc_client_id' field is present in the system profile.
        6.  The 'rhc_client_id' in the system profile is identical to the
            subscription manager UUID within the timeout period.
    """

    with contextlib.suppress(Exception):
        rhc.disconnect()
    command_args = prepare_args_for_connect(test_config, auth=auth)
    command = ["connect"] + command_args
    rhc.run(*command)
    assert rhc.is_registered
    assert yggdrasil_service_is_active()
    timeout = 60.0
    start = time.time()
    while True:
        system_profile = external_inventory.this_system_profile()
        if "rhc_client_id" in system_profile:
            assert system_profile["rhc_client_id"] == str(subman.uuid)
            break
        current = time.time()
        if current - start > timeout:
            raise ValueError("timeout")
        time.sleep(10)
