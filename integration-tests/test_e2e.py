import contextlib
import pytest
from time import sleep
from utils import (
    yggdrasil_service_is_active,
    prepare_args_for_connect,
)


@pytest.mark.parametrize("auth", ["activation-key", "basic"])
def test_rhc_client_connect_e2e(rhc, test_config, auth, external_inventory, subman):
    """
    Test RHC Client connect e2e using basic auth and activation key
    test_steps:
        1. Run RHC client connect
    expected_results:
        1. The rhcd service is running and active
        2. A new host is created on Inventory
        3. The host rhc client id matches the rhc client id collected from the machine
    """
    with contextlib.suppress(Exception):
        rhc.disconnect()
    command_args = prepare_args_for_connect(test_config, auth=auth)
    command = ["connect"] + command_args
    rhc.run(*command)
    assert rhc.is_registered
    assert yggdrasil_service_is_active()
    sleep(30)  # small wait time for inventory sync up
    system_profile = external_inventory.this_system_profile()
    assert system_profile["rhc_client_id"] == str(subman.uuid)
