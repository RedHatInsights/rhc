"""
This Python module contains integration tests for rhc.
It uses pytest-client-tools Python module.More information about this
module could be found: https://github.com/RedHatInsights/pytest-client-tools/
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
    Test RHC Client connect e2e using basic auth and activation key
    test_steps:
        1. Run RHC client connect
    expected_results:
        1. The rhcd service is running and active
        2. A new host is created in Inventory
        3. The rhc_client_id in system profile matches the subman-id
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
