"""
This Python module contains integration tests for rhc.
It uses pytest-client-tools Python module.More information about this
module could be found: https://github.com/RedHatInsights/pytest-client-tools/
"""

import contextlib
import time
import logging
import pytest
import subprocess
from datetime import datetime
import sh
from pytest_client_tools.restclient import RestClient

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


@pytest.mark.skipif(
    pytest.rhel_major_version == "unknown" or int(pytest.rhel_major_version) < 9,
    reason="This test is only supported on RHEL 9 and above",
)
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
        is registered, rhcd service is active, and that repositories from the content
        template appear in the dnf repolist after connection.
    :tags: Tier 1
    :steps:
        1.  Get initial dnf repolist before connection.
        2.  Run the 'rhc connect' command with content template.
        3.  Verify that RHC reports being registered.
        4.  Verify that the rhcd service is active.
        5.  Verify that dnf repolist contains new repository after connection.
    :expectedresults:
        1.  Initial dnf repolist is captured.
        2.  The 'rhc connect' command executes without error.
        3.  RHC indicates the system is registered.
        4.  The rhcd service is in an active state.
        5.  A new repository appears in dnf repolist after connection.
    """
    # rhc+satellite does not support content template
    if "satellite" in test_config.environment:
        pytest.skip("Content template not supported in satellite environment")

    # Ensure system is disconnected
    with contextlib.suppress(Exception):
        rhc.disconnect()

    # Build proxy configuration for stage environment
    proxies = None
    if test_config.environment == "stage":
        proxy_host = test_config.get("noauth_proxy.host")
        proxy_port = test_config.get("noauth_proxy.port")
        proxy_url = f"http://{proxy_host}:{proxy_port}"
        proxies = {"http": proxy_url, "https": proxy_url}

    template_name = test_config.get("rhc.template_name_el9")
    template_repos = get_template_repos_by_name(
        template_name,
        test_config.get("candlepin.username"),
        test_config.get("candlepin.password"),
        test_config.get("rhc.template_url"),
        proxies=proxies,
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
    assert rhcd_service_is_active()

    # Get repository list after connection
    final_repo_names = set(get_enabled_repo_names())

    # Validate at least one template repo appeared in system repos.
    appeared_repos = template_repos & (final_repo_names - initial_repo_names)
    assert (
        appeared_repos
    ), "No new repositories appeared in the system after connecting the content template"


@pytest.mark.skipif(
    pytest.rhel_major_version == "unknown" or int(pytest.rhel_major_version) < 9,
    reason="This test is only supported on RHEL 9 and above",
)
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
        5.  Verify that the rhcd service is not active.
    :expectedresults:
        1.  The system is successfully disconnected (if previously connected).
        2.  The 'rhc connect' command fails.
        3.  The error message indicates the content template could not be found.
        4.  The system is not registered with RHC.
        5.  The rhcd service is not in an active state.
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
    assert not rhcd_service_is_active()

    # Verify the specific error message
    expected_error = f'Environment with name "{nonexistent_CTT}" could not be found.'
    assert expected_error in result.stderr or expected_error in result.stdout


def get_enabled_repo_names():
    """
    Return a set of enabled DNF repository names (not IDs).
    """
    repo_names = set()
    output = subprocess.check_output(["dnf", "repolist", "--enabled"], universal_newlines=True)
    for line in output.splitlines():
        line = line.strip()
        if line and not line.startswith("repo id"):
            parts = line.split(maxsplit=1)  # split into 2 parts: id and name
            if len(parts) == 2:
                repo_names.add(parts[1].strip())  # take the name part
    return repo_names


def get_template_repos_by_name(
    template_name, username, password, template_url, proxies=None
):
    """
    Fetch a content template by name from Red Hat Console and return
    the set of repository names in its snapshots.

    Args:
        template_name: Name of the content template to find
        username: Username for authentication
        password: Password for authentication
        template_url: Base URL for the templates API
        proxies: Optional dict of proxies, e.g. {"http": "...", "https": "..."}
    """
    client = RestClient(base_url=template_url, verify=True, proxies=proxies)

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
