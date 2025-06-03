"""
This Python module contains integration tests for rhc.
It uses pytest-client-tools Python module.More information about this
module could be found: https://github.com/RedHatInsights/pytest-client-tools/
"""

import contextlib
import time
import logging
import pytest
from datetime import datetime
import sh

from utils import (
    rhcd_service_is_active,
    prepare_args_for_connect,
    check_rhcd_journalctl_logs,
)
import re

logger = logging.getLogger(__name__)


@pytest.mark.parametrize("auth", ["basic", "activation-key"])
def test_connect(external_candlepin, rhc, test_config, auth):
    """Test if RHC can connect to CRC using basic auth and activation key,
    Also verify that rhcd service is in active state afterward.
    """
    # rhc+satellite does not support basic auth for now
    # refer: https://issues.redhat.com/browse/RHEL-53436
    if "satellite" in test_config.environment and auth == "basic":
        pytest.skip("rhc+satellite only support activation key registration now")
    with contextlib.suppress(Exception):
        rhc.disconnect()
    command_args = prepare_args_for_connect(test_config, auth=auth)
    command = ["connect"] + command_args
    result = rhc.run(*command)
    assert rhc.is_registered
    assert rhcd_service_is_active()
    assert "Connected to Red Hat Subscription Management" in result.stdout
    assert "Connected to Red Hat Insights" in result.stdout
    # copr builds have BrandName rhc and downstream builds have "Remote Host Configuration"
    assert re.search(
        r"(Activated the Remote Host Configuration daemon|Activated the rhc daemon)",
        result.stdout,
    )
    assert "Successfully connected to Red Hat!" in result.stdout


@pytest.mark.parametrize(
    "credentials",
    [
        # password : valid, username: invalid
        {"username": "non-existent-user", "password": "candlepin.password"},
        # activation-key : valid, organization: invalid
        {
            "organization": "non-existent-org",
            "activation-key": "candlepin.activation_keys",
        },
        # username : valid, password: invalid
        {"username": "candlepin.username", "password": "xpto123"},
        # organization : valid, activation-key: invalid
        {"organization": "candlepin.org", "activation-key": "xpto123"},
        # invalid combination of parameters (username & activation-key)
        {
            "username": "candlepin.username",
            "activation-key": "candlepin.activation_keys",
        },
        # invalid combination of parameters (password & activation-key)
        {
            "activation-key": "candlepin.activation_keys",
            "password": "candlepin.password",
        },
        # invalid combination of parameters (activation-key without organization)
        {
            "activation-key": "candlepin.activation_keys",
        },
    ],
)
def test_connect_wrong_parameters(external_candlepin, rhc, test_config, credentials):
    """Test if RHC handles invalid credentials properly"""
    # An attempt to bring system in disconnected state in case it is not.
    with contextlib.suppress(Exception):
        rhc.disconnect()
    command_args = prepare_args_for_connect(test_config, credentials=credentials)

    command = ["connect"] + command_args
    result = rhc.run(*command, check=False)
    assert result.returncode == 1
    assert not rhcd_service_is_active()
