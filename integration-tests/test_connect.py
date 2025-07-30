"""
This Python module contains integration tests for rhc.
It uses pytest-client-tools Python module.More information about this
module could be found: https://github.com/RedHatInsights/pytest-client-tools/
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

@pytest.mark.tier1
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
    :id: e74695bf-384c-4d9f-aeb4-2348027052dc
    :title: Verify successful RHC connection using basic auth and activation key
    :parametrized: yes
    :description:
        This test verifies that RHC can successfully connect to CRC using either
        basic authentication or an activation key. It also checks that the
        yggdrasil/rhcd service is in active state after a successful
        connection. The test covers both default text output and machine-readable
        JSON output formats.
    :tags: Tier 1
    :steps:
        1.  Ensure the system is disconnected from RHC.
        2.  Run the 'rhc connect' command using the given authentication method and output format.
        3.  Verify that RHC reports being registered.
        4.  Verify that the yggdrasil or rhcd service is active.
        5.  Verify the command output based on the specified format (text or JSON).
    :expectedresults:
        1.  The system is successfully disconnected (if previously connected).
        2.  The 'rhc connect' command executes without error.
        3.  RHC indicates the system is registered.
        4.  The yggdrasil/rhcd service is in an active state.
        5.  For text output, stdout contains "Connected to Red Hat Insights",
            "Connected to Red Hat Subscription Management",
            "Activated the yggdrasil service" or "Activated the Remote Host Configuration daemon"
            and "Successfully connected to Red Hat!".
            For JSON output, no specific assertions are made due to a known issue (CCT-1191).
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
    "credentials,return_code",
    [
        (  # username: invalid, password: valid
            {
                "username": "non-existent-user",
                "password": "candlepin.password"
            },
            None,
        ),
        (  # username: valid, password: invalid
            {
                "username": "candlepin.username",
                "password": "xpto123"
            },
            None,
        ),
        (  # organization: invalid, activation-key: valid
            {
                "organization": "non-existent-org",
                "activation-key": "candlepin.activation_keys",
            },
            None,
        ),
        (  # organization: valid, activation-key: invalid
            {
                "organization": "candlepin.org",
                "activation-key": "xpto123"
            },
            None,
        ),
        (  # invalid combination of parameters (username & activation-key)
            {
                "username": "candlepin.username",
                "activation-key": "candlepin.activation_keys",
            },
            64,
        ),
        (  # invalid combination of parameters (password & activation-key)
            {
                "activation-key": "candlepin.activation_keys",
                "password": "candlepin.password",
            },
            64,
        ),
        (  # invalid combination of parameters (activation-key without organization)
            {
                "activation-key": "candlepin.activation_keys",
            },
            64,
        ),
    ],
)
def test_connect_wrong_parameters(
    external_candlepin, rhc, test_config, credentials, return_code
):
    """
    :id: 9631c021-72a1-4030-90d7-8d14bd3d1304
    :title: Verify RHC connect handles invalid parameters and credentials properly
    :parametrized: yes
    :description:
        This test verifies that the 'rhc connect' command correctly handles various
        scenarios involving invalid credentials (wrong username/password or
        organization/activation key) and invalid parameter combinations (e.g.,
        using both username and activation key). It checks that the command fails
        and the yggdrasil/rhcd service does not become active.
    :tags:
    :steps:
        1.  Ensure the system is disconnected from RHC.
        2.  Run the 'rhc connect' command using invalid credentials or parameters,
            expecting it to fail.
        3.  Verify the command's return code.
        4.  Verify that the yggdrasil/rhcd service is not active.
    :expectedresults:
        1.  The system is successfully disconnected (if previously connected).
        2.  The 'rhc connect' command fails.
        3.  The command's return code matches the expected non-zero value
            (or a specific code if provided).
        4.  The yggdrasil/rhcd service is not in an active state.
    """

    # An attempt to bring system in disconnected state in case it is not.
    with contextlib.suppress(Exception):
        rhc.disconnect()
    command_args = prepare_args_for_connect(
        test_config, credentials=credentials
    )
    command = ["connect"] + command_args
    result = rhc.run(*command, check=False)
    if return_code is not None:
        assert result.returncode == return_code
    else:
        assert result.returncode != 0
    assert not yggdrasil_service_is_active()


@pytest.mark.skip("Test cannot be run due to unresolved issues CCT-696")
@pytest.mark.parametrize("auth", ["basic", "activation-key"])
def test_rhc_worker_playbook_install_after_rhc_connect(
    external_candlepin, rhc, test_config, auth
):
    """
    :id: a86b4dea-77c4-4c5e-8412-a7eb0f1342ab
    :title: Verify rhc-worker-playbook is installed after RHC connection
    :parametrized: yes
    :description:
        This test verifies that the 'rhc-worker-playbook' package is automatically
        installed after successfully connecting RHC, regardless of whether basic
        authentication or an activation key is used. It monitors service logs to
        confirm the installation process and logs the time taken.
    :tags:
    :steps:
        1.  Remove the 'rhc-worker-playbook' package if it is installed.
        2.  Ensure the system is disconnected from RHC.
        3.  Run the 'rhc connect' command using the specified authentication method.
        4.  Verify that the 'rhc-worker-playbook' package is installed.
    :expectedresults:
        1.  The 'rhc-worker-playbook' package is successfully removed.
        2.  The system is successfully disconnected (if previously connected).
        3.  The 'rhc connect' command executes successfully, and RHC reports being registered.
        4.  The 'rhc-worker-playbook' package is installed.
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
