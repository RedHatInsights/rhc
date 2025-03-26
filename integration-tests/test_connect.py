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
    yggdrasil_service_is_active,
    prepare_args_for_connect,
    check_yggdrasil_journalctl_logs,
)

logger = logging.getLogger(__name__)


@pytest.mark.parametrize(
    "auth, output_format",
    [
        ("basic", None),
        ("basic", "json"),
        ("activation-key", None),
        ("activation-key", "json"),
    ]
)
def test_connect(external_candlepin, rhc, test_config, auth, output_format):
    """
     Test if RHC can connect to CRC using basic auth and activation key,
     Also verify that yggdrasil/rhcd service is in active state afterward.
     Two variants of output format is considered.
       * Default text output
       * Machine readable output (JSON document)
     """
    # rhc+satellite does not support basic auth for now
    # refer: https://issues.redhat.com/browse/RHEL-53436
    if "satellite" in test_config.environment and auth == "basic":
        pytest.skip("rhc+satellite only support activation key registration now")
    with contextlib.suppress(Exception):
        rhc.disconnect()
    command_args = prepare_args_for_connect(test_config, auth=auth, output_format=output_format)
    command = ["connect"] + command_args
    result = rhc.run(*command)
    assert rhc.is_registered
    assert yggdrasil_service_is_active()

    if output_format is None:
        assert "Connected to Red Hat Subscription Management" in result.stdout
        assert "Connected to Red Hat Insights" in result.stdout
    elif output_format == "json":
        pass
        # TODO: parse result.stdout, when CCT-1191 is fixed. It is not possible now, because
        #       "rhc connect --format json" prints JSON document to stderr (not stdout)
        # json_output = json.loads(result.stdout)
        # assert json_output["rhsm_connected"] is True

    if pytest.service_name == "rhcd":
        if output_format is None:
            assert "Activated the Remote Host Configuration daemon" in result.stdout
        elif output_format == "json":
            pass
            # TODO: parse result.stdout, when CCT-1191 is fixed
    else:
        if output_format is None:
            assert "Activated the yggdrasil service" in result.stdout
        elif output_format == "json":
            pass
            # TODO: parse result.stdout, when CCT-1191 is fixed

    if output_format is None:
        assert "Successfully connected to Red Hat!" in result.stdout
    elif output_format == "json":
        pass
        # TODO: parse result.stdout, when CCT-1191 is fixed


@pytest.mark.parametrize(
    "credentials,server",
    [
        (  # username and password: valid, server: invalid
            {"username": "candlepin.username", "password": "candlepin.password"},
            "http://non-existent.server",
        ),
        (  # organization and activation-key: valid, server: invalid
            {
                "organization": "candlepin.org",
                "activation-key": "candlepin.activation_keys",
            },
            "http://non-existent.server",
        ),
        (  # password and server: valid, username: invalid
            {"username": "non-existent-user", "password": "candlepin.password"},
            None,
        ),
        (  # activation-key and server: valid, organization: invalid
            {
                "organization": "non-existent-org",
                "activation-key": "candlepin.activation_keys",
            },
            None,
        ),
        (  # username and server: valid, password: invalid
            {"username": "candlepin.username", "password": "xpto123"},
            None,
        ),
        (  # organization and server: valid, activation-key: invalid
            {"organization": "candlepin.org", "activation-key": "xpto123"},
            None,
        ),
        # invalid combination of parameters
        pytest.param(
            {
                "username": "candlepin.username",
                "activation-key": "candlepin.activation_keys",
            },
            None,
            marks=pytest.mark.xfail(
                reason="Unresolved Bug https://issues.redhat.com/browse/CCT-1155"
            ),
        ),
        pytest.param(
            {
                "activation-key": "candlepin.activation_keys",
                "password": "candlepin.password",
            },
            None,
            marks=pytest.mark.xfail(
                reason="Unresolved Bug https://issues.redhat.com/browse/CCT-1155"
            ),
        ),
    ],
)
def test_connect_wrong_parameters(
    external_candlepin, rhc, test_config, credentials, server
):
    """Test if RHC handles invalid credentials properly"""
    # An attempt to bring system in disconnected state in case it is not.
    with contextlib.suppress(Exception):
        rhc.disconnect()
    command_args = prepare_args_for_connect(
        test_config, credentials=credentials, server=server
    )
    command = ["connect"] + command_args
    result = rhc.run(*command, check=False)
    assert result.returncode == 1
    assert not yggdrasil_service_is_active()


@pytest.mark.skip("Test cannot be run due to unresolved issues CCT-696")
@pytest.mark.parametrize("auth", ["basic", "activation-key"])
def test_rhc_worker_playbook_install_after_rhc_connect(
    external_candlepin, rhc, test_config, auth
):
    """
    Test that rhc-worker-playbook is installed after rhc-connect,
    Also log the total time taken to install the package
        test_steps:
            1- run 'rhc connect'
            2- monitor yggdrasil/rhcd logs to see when package-manager-worker installs 'rhc-worker-playbook'
            3- validate rhc-worker-playbook is installed
    """
    with contextlib.suppress(Exception):
        sh.yum("remove", "rhc-worker-playbook", "-y")

    success_message = "Registered rhc-worker-playbook"
    with contextlib.suppress(Exception):
        rhc.disconnect()

    start_date_time = datetime.now().strftime(
        "%Y-%m-%d %H:%M:%S"
    )  # current date and time for observing yggdrasil/rhcd logs
    command_args = prepare_args_for_connect(test_config, auth=auth)
    command = ["connect"] + command_args
    rhc.run(*command, check=False)
    assert rhc.is_registered

    # Verifying if rhc-worker-playbook was installed successfully
    t_end = time.time() + 60 * 5  # maximum time to wait for installation
    while time.time() < t_end:
        installed_status = check_yggdrasil_journalctl_logs(
            str_to_check=success_message,
            since_datetime=start_date_time,
            must_exist_in_log=True,
        )
        if installed_status:
            break
    assert (
        installed_status
    ), "rhc connect is expected to install rhc_worker_playbook package"

    total_runtime = datetime.now() - datetime.strptime(
        start_date_time, "%Y-%m-%d %H:%M:%S"
    )
    pkg_version = sh.rpm("-qa", "rhc-worker-playbook")
    logger.info(f"successfully installed rhc_worker_playbook package {pkg_version}")
    logger.info(
        f"time taken to start yggdrasil/rhcd service and install "
        f"rhc_worker_playbook : {total_runtime} s"
    )
