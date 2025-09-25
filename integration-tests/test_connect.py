"""
:casecomponent: rhc
:requirement: RHSS-291300
:subsystemteam: rhel-sst-csi-client-tools
:caseautomation: Automated
:upstream: Yes
"""

import contextlib
import json
import time
import logging
import pytest
from datetime import datetime
import sh
import subprocess
from pytest_client_tools.restclient import RestClient

from utils import (
    yggdrasil_service_is_active,
    prepare_args_for_connect,
    check_yggdrasil_journalctl_logs,
    configure_proxy_rhsm

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
        yggdrasil service is in active state after a successful
        connection. The test covers both default text output and machine-readable
        JSON output formats.
    :tags: Tier 1
    :steps:
        1.  Ensure the system is disconnected from RHC.
        2.  Run the 'rhc connect' command using the given authentication method and output format.
        3.  Verify that RHC reports being registered.
        4.  Verify that the yggdrasil service is active.
        5.  Verify the command output based on the specified format (text or JSON).
    :expectedresults:
        1.  The system is successfully disconnected (if previously connected).
        2.  The 'rhc connect' command executes without error.
        3.  RHC indicates the system is registered.
        4.  The yggdrasil service is in an active state.
        5.  For text output, stdout contains "Connected to Red Hat Insights",
            "Connected to Red Hat Subscription Management",
            "Activated the yggdrasil service" and "Successfully connected to Red Hat!".
            For JSON output, comprehensive validation is performed on the response structure and values.
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
        # Verify connection messages
        assert "Connected to Red Hat Subscription Management" in result.stdout
        assert "Connected to Red Hat Insights" in result.stdout
        assert "Activated the yggdrasil service" in result.stdout

        # Verify final success message
        assert "Successfully connected to Red Hat!" in result.stdout

    elif output_format == "json":
        json_output = json.loads(result.stdout)

        # Verify field types and values
        assert type(json_output["hostname"]) == str
        assert type(json_output["uid"]) == int
        assert (
            type(json_output["rhsm_connected"]) == bool
            and json_output["rhsm_connected"] is True
        )
        assert type(json_output["features"]) == dict

        # Verify feature types and values
        features = json_output["features"]
        for feature_name in ["content", "analytics", "remote_management"]:
            for key in ["enabled", "successful"]:
                value = features[feature_name][key]
                assert (
                    isinstance(value, bool) and value is True
                ), f"{feature_name}.{key} should be True boolean, got {value!r}"

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
        and the yggdrasil service does not become active.
    :tags:
    :steps:
        1.  Ensure the system is disconnected from RHC.
        2.  Run the 'rhc connect' command using invalid credentials or parameters,
            expecting it to fail.
        3.  Verify the command's return code.
        4.  Verify that the yggdrasil service is not active.
    :expectedresults:
        1.  The system is successfully disconnected (if previously connected).
        2.  The 'rhc connect' command fails.
        3.  The command's return code matches the expected non-zero value
            (or a specific code if provided).
        4.  The yggdrasil service is not in an active state.
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
    )  # current date and time for observing yggdrasil logs
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
        f"time taken to start yggdrasil service and install "
        f"rhc_worker_playbook : {total_runtime} s"
    )


@pytest.mark.parametrize("auth_proxy", [False, True])
def test_connect_proxy(
    external_candlepin,
    subman,
    insights_client,
    rhc,
    test_config,
    yggdrasil_proxy_config,
    auth_proxy,
):
    """
    :id: 8f2a3b1c-4d5e-6f7g-8h9i-0j1k2l3m4n5o
    :title: Verify successful RHC connection through proxy (authenticated and non-authenticated)
    :parametrized: yes
    :description:
        This test verifies that RHC can successfully connect to CRC through both
        authenticated and non-authenticated proxy configurations. It configures
        subscription-manager, insights-client, and yggdrasil service to use the
        specified proxy settings and verifies that the connection is established
        and the yggdrasil service becomes active.
    :tags: Tier 1
    :steps:
        1.  Ensure the system is disconnected from RHC.
        2.  Configure subscription-manager with proxy settings.
        3.  Configure insights-client with proxy settings.
        4.  Configure yggdrasil service with proxy environment variables.
        5.  Run the 'rhc connect' command.
        6.  Verify that RHC reports being registered.
        7.  Verify that the yggdrasil service is in active state.
    :expectedresults:
        1.  The system is successfully disconnected from RHC.
        2.  Subscription-manager proxy settings are configured correctly.
        3.  Insights-client proxy settings are configured correctly.
        4.  Yggdrasil service proxy environment variables are set.
        5.  'rhc connect' command executes successfully.
        6.  The system is registered with RHC.
        7.  The yggdrasil service is in active state.
    """
    with contextlib.suppress(Exception):
        rhc.disconnect()

    # Configure proxy in rhsm.conf
    proxy_url = configure_proxy_rhsm(subman, test_config, auth_proxy=auth_proxy)

    # Configure proxy in insights-client.conf
    insights_client.config.proxy = proxy_url
    insights_client.config.save()

    # Configure yggdrasil service proxy in systemd override file
    yggdrasil_proxy_config(proxy_url)

    rhc.connect(
        username=test_config.get("candlepin.username"),
        password=test_config.get("candlepin.password"),
    )
    # validate the connection
    assert rhc.is_registered
    assert yggdrasil_service_is_active()


@pytest.mark.tier1
@pytest.mark.parametrize("auth", ["basic", "activation-key"])
def test_connect_with_content_template(external_candlepin, rhc, test_config, auth):
    """
    :id: c45b67a8-123d-4e5f-6789-0123456789ab
    :title: Verify 'rhc connect' while specifying a content-template,
            and receive content within that template
    :parametrized: yes
    :description:
        This test verifies that RHC can successfully connect using a content template
        with both basic authentication and activation key. It verifies that the system
        is registered, yggdrasil service is active, and that repositories from the content
        template appear in the dnf repolist after connection.
    :tags: Tier 1
    :steps:
        1.  Get initial dnf repolist before connection.
        2.  Run the 'rhc connect' command with content template.
        3.  Verify that RHC reports being registered.
        4.  Verify that the yggdrasil service is active.
        5.  Verify that dnf repolist contains new repository after connection.
    :expectedresults:
        1.  Initial dnf repolist is captured.
        2.  The 'rhc connect' command executes without error.
        3.  RHC indicates the system is registered.
        4.  The yggdrasil service is in an active state.
        5.  A new repository appears in dnf repolist after connection.
    """
    # rhc+satellite does not support basic auth for now
    # refer: https://issues.redhat.com/browse/RHEL-53436
    if "satellite" in test_config.environment:
        pytest.skip("Content template not supported in satellite environment")

    # Ensure system is disconnected
    with contextlib.suppress(Exception):
        rhc.disconnect()

    template_name = test_config.get("rhc.template_name")
    template_repos = get_template_repos_by_name(
        template_name,
        test_config.get("candlepin.username"),
        test_config.get("candlepin.password"),
        test_config.get("rhc.template_url"),
    )

    initial_repo_names = get_enabled_repo_names()

    # Execute rhc connect with content template
    command_args = prepare_args_for_connect(
        test_config, auth=auth, content_template=template_name
    )

    command = ["connect"] + command_args
    result = rhc.run(*command)

    # Verify connection
    assert rhc.is_registered
    assert yggdrasil_service_is_active()

    # Get repository list after connection
    final_repo_names = set(get_enabled_repo_names())

    # Validate at least one template repo appeared in system repos.
    # Note: some GA repositories may be missing on test systems due to beta/production certificate differences.
    appeared_repos = template_repos & (final_repo_names - initial_repo_names)
    assert (
        appeared_repos
    ), "No new repositories appeared in the system after connecting the content template"


def test_connect_with_nonexistent_content_template(
    external_candlepin, rhc, test_config
):
    """
    :id: d56e78b9-234e-5f6g-7890-1234567890cd
    :title: Verify 'rhc connect' with non-existent content-template throws appropriate error
    :description:
        This test verifies that RHC connect command fails with appropriate error message
        when a non-existent content template is specified.
    :tags:
    :steps:
        1.  Ensure the system is disconnected from RHC.
        2.  Run the 'rhc connect' command with a non-existent content template.
        3.  Verify that the command fails with the expected error message.
        4.  Verify that the system remains unregistered.
        5.  Verify that the yggdrasil service is not active.
    :expectedresults:
        1.  The system is successfully disconnected (if previously connected).
        2.  The 'rhc connect' command fails.
        3.  The error message indicates the content template could not be found.
        4.  The system is not registered with RHC.
        5.  The yggdrasil service is not in an active state.
    """
    if "satellite" in test_config.environment:
        pytest.skip("Content template not supported in satellite environment")

    # Ensure system is disconnected
    with contextlib.suppress(Exception):
        rhc.disconnect()

    # Use a non-existent content template
    nonexistent_CTT = "nonexistent_CTT"

    # Execute rhc connect with non-existent content template using activation-key
    command_args = prepare_args_for_connect(
        test_config, auth="activation-key", content_template=nonexistent_CTT
    )
    command = ["connect"] + command_args
    result = rhc.run(*command, check=False)

    # Verify connection failed
    assert result.returncode != 0
    assert not rhc.is_registered
    assert not yggdrasil_service_is_active()

    # Verify the specific error message
    expected_error = f'Environment with name "{nonexistent_CTT}" could not be found.'
    assert expected_error in result.stderr or expected_error in result.stdout


def get_enabled_repo_names():
    """
    Return a set of enabled DNF repository names (not IDs).
    """
    repo_names = set()
    output = subprocess.check_output(["dnf", "repolist", "--enabled"], text=True)
    for line in output.splitlines():
        line = line.strip()
        if line and not line.startswith("repo id"):
            parts = line.split(maxsplit=1)  # split into 2 parts: id and name
            if len(parts) == 2:
                repo_names.add(parts[1].strip())  # take the name part
    return repo_names


def get_template_repos_by_name(template_name, username, password, template_url):
    """
    Fetch a content template by name from Red Hat Console and return
    the set of repository names in its snapshots.
    """

    client = RestClient(base_url=template_url, verify=True)

    try:
        # Get all templates and find the one matching template_name
        response = client.get(
            "templates/",
            auth=(username, password),
            headers={"accept": "application/json"},
        )

        templates_data = response.json()

        # Handle the API response structure: {"data": [...]}
        if isinstance(templates_data, dict) and "data" in templates_data:
            templates = templates_data["data"]
        elif isinstance(templates_data, list):
            templates = templates_data
        else:
            raise ValueError(f"Unexpected API response format: {type(templates_data)}")

        # Find the template by name
        for template in templates:
            if isinstance(template, dict) and template.get("name") == template_name:
                template_id = template.get("uuid")

                # Fetch template details by ID
                detail_resp = client.get(
                    f"templates/{template_id}",
                    auth=(username, password),
                    headers={"accept": "application/json"},
                )
                template_details = detail_resp.json()

                # Extract repository names from snapshots
                snapshots = template_details.get("snapshots", [])
                repository_names = set()

                for snapshot in snapshots:
                    if isinstance(snapshot, dict) and "repository_name" in snapshot:
                        repository_names.add(snapshot["repository_name"])

                return repository_names

        # List available templates for debugging
        available_templates = [
            t.get("name", "Unknown") for t in templates if isinstance(t, dict)
        ]
        raise ValueError(
            f"Content template '{template_name}' not found. Available templates: {available_templates}"
        )

    except Exception as e:
        print(f"Error fetching template: {e}")
        if "response" in locals():
            print(f"Response status: {response.status_code}")
            print(f"Response content (first 200 chars): {response.text[:200]}...")
        raise
