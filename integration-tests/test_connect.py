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
from itertools import combinations
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

# ============================================================================
# Centralized Feature Definitions
# ============================================================================
# To add a new feature:
# 1. Add mapping entry in FEATURE_MAPPING (CLI name -> JSON name)
#    (the order must be the same as in the --help message)
# 2. If the feature has dependencies, add them to FEATURE_DEPENDENCIES
# 3. The tests will automatically include the new feature in parameterized tests
# ============================================================================

# Map CLI feature names to JSON feature names
FEATURE_MAPPING = {
    "content": "content",
    "analytics": "analytics",
    "remote-management": "remote_management",
}

# List of all valid features (CLI names)
ALL_FEATURES_CLI = list(FEATURE_MAPPING.keys())

# List of all valid features (JSON names)
ALL_FEATURES_JSON = list(FEATURE_MAPPING.values())

# Feature dependencies: feature -> list of required features (CLI names)
# Example: remote-management requires both content and analytics to be enabled
FEATURE_DEPENDENCIES = {
    "remote-management": ["content", "analytics"],
}


def get_dependent_features(feature):
    """
    Get the list of features that depend on the given feature.
    """
    dependent_features = []
    for feat, deps in FEATURE_DEPENDENCIES.items():
        if feature in deps:
            dependent_features.append(feat)
    return dependent_features


def get_required_features(feature):
    """
    Get the list of features required by the given feature.
    """
    return FEATURE_DEPENDENCIES.get(feature, [])


def generate_feature_combinations():
    """
    Generate all possible feature enable/disable combinations with expected outcomes.
    Returns:
        List of tuples: (enabled_features, disabled_features, expected_states, should_fail)
    """
    test_cases = []

    # Generate all possible subsets of features (from empty set to all features)
    for r in range(len(ALL_FEATURES_CLI) + 1):
        for enabled_subset in combinations(ALL_FEATURES_CLI, r):
            enabled_features = list(enabled_subset)
            disabled_features = [f for f in ALL_FEATURES_CLI if f not in enabled_features]

            # Determine if this combination should fail
            # It fails if any feature with dependencies is enabled but not all its dependencies are enabled
            should_fail = False
            for feature in enabled_features:
                required = get_required_features(feature)
                if required and not all(req in enabled_features for req in required):
                    should_fail = True
                    break

            if should_fail:
                expected_states = None
            else:
                expected_states = {
                    FEATURE_MAPPING[feat]: feat in enabled_features
                    for feat in ALL_FEATURES_CLI
                }

            test_cases.append((enabled_features, disabled_features, expected_states, should_fail))

    return test_cases


def is_remote_management_fully_enabled(enabled_features):
    """
    Check if remote-management is enabled with all its dependencies satisfied.
    """
    if "remote-management" not in enabled_features:
        return False
    required = get_required_features("remote-management")
    return all(req in enabled_features for req in required)


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
        for feature_name in ALL_FEATURES_JSON:
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

    # Build proxy configuration for stage environment
    proxies = None
    if test_config.environment == "stage":
        proxy_host = test_config.get("noauth_proxy.host")
        proxy_port = test_config.get("noauth_proxy.port")
        proxy_url = f"http://{proxy_host}:{proxy_port}"
        proxies = {"http": proxy_url, "https": proxy_url}

    template_name = test_config.get("rhc.template_name")
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


def get_template_repos_by_name(template_name, username, password, template_url, proxies=None):
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


@pytest.mark.tier1
@pytest.mark.parametrize("feature", ALL_FEATURES_CLI)
def test_connect_with_single_feature_disabled(external_candlepin, rhc, test_config, feature):
    """
    :id: 4eb2000d-398a-404b-a3c4-56b2cab91a62
    :title: Verify RHC connection with one feature disabled using --disable-feature flag
    :parametrized: yes
    :description:
        This test verifies that when connecting to RHC with one feature explicitly
        disabled using the --disable-feature flag, the expected features are disabled
        based on dependency rules. Features that depend on the disabled feature will
        also be automatically disabled. The test validates this by examining the JSON
        output of the connect command.
    :tags: Tier 1
    :steps:
        1.  Ensure the system is disconnected from RHC.
        2.  Run the 'rhc connect' command with --disable-feature flag for the specified feature and JSON output format.
        3.  Verify that RHC reports being registered.
        4.  Verify that the yggdrasil service is inactive.
        5.  Parse the JSON output and verify the disabled feature shows enabled=False.
        6.  Verify that features dependent on the disabled feature are also disabled.
        7.  Verify that independent features remain enabled.
    :expectedresults:
        1.  The system is successfully disconnected (if previously connected).
        2.  The 'rhc connect' command executes without error.
        3.  RHC indicates the system is registered.
        4.  The yggdrasil service is not in an active state.
        5.  The explicitly disabled feature has enabled=False in the JSON output.
        6.  Dependent features are automatically disabled based on dependency rules.
        7.  Independent features remain enabled and show the correct status.
    """
    with contextlib.suppress(Exception):
        rhc.disconnect()

    command_args = prepare_args_for_connect(test_config, auth="activation-key", output_format="json")

    command_args.extend(["--disable-feature", feature])

    command = ["connect"] + command_args

    result = rhc.run(*command)

    assert rhc.is_registered
    assert not yggdrasil_service_is_active()

    json_output = json.loads(result.stdout)
    features = json_output["features"]

    disabled_feature_key = FEATURE_MAPPING[feature]

    # Verify the disabled feature has enabled=false
    assert features[disabled_feature_key]["enabled"] is False, (
        f"Feature '{disabled_feature_key}' should be disabled")

    # Check which features depend on the disabled feature (should also be disabled)
    dependent_features = get_dependent_features(feature)
    for dependent_feature in dependent_features:
        dependent_feature_json = FEATURE_MAPPING[dependent_feature]
        assert features[dependent_feature_json]["enabled"] is False, (
            f"Feature '{dependent_feature_json}' should be disabled when '{feature}' is disabled "
            f"because it depends on '{feature}'")

    # Verify that features not disabled and not dependent on disabled feature remain enabled
    required_features = get_required_features(feature)
    for feat_cli in ALL_FEATURES_CLI:
        feat_json = FEATURE_MAPPING[feat_cli]
        # Skip the disabled feature and its dependents
        if feat_cli == feature or feat_cli in dependent_features:
            continue
        # If this is a required feature for the disabled one, it should still be enabled
        if feat_cli in required_features or feat_cli not in dependent_features:
            assert features[feat_json]["enabled"] is True, (
                f"Feature '{feat_json}' should remain enabled when '{feature}' is disabled")


@pytest.mark.tier1
# Note: Test cases are automatically generated from FEATURE_MAPPING and FEATURE_DEPENDENCIES.
# When adding a new feature:
# 1. Add the feature to FEATURE_MAPPING
# 2. If it has dependencies, add them to FEATURE_DEPENDENCIES
# 3. The test cases will be automatically generated to cover all combinations
@pytest.mark.parametrize(
    "enabled_features,disabled_features,expected_states,should_fail",
    generate_feature_combinations(),
)
def test_connect_with_feature_enabled_disabled_combinations(
    external_candlepin, rhc, test_config, enabled_features, disabled_features,
    expected_states, should_fail
):
    """
    :id: 64798f07-709d-4ad6-ad5d-9423602803bf
    :title: Verify RHC connection with various feature enable/disable combinations
    :parametrized: yes
    :description:
        This test verifies 'rhc connect' behavior with --enable-feature and --disable-feature
        flags for all available features. Test cases are automatically generated from
        FEATURE_MAPPING and FEATURE_DEPENDENCIES to cover all possible combinations.
        Key rule: Features with dependencies (e.g., remote-management requires both
        content AND analytics enabled) will cause connection failure if dependencies
        are not met.
    :tags: Tier 1
    :steps:
        1.  Ensure the system is disconnected from RHC.
        2.  Run 'rhc connect' with --enable-feature and --disable-feature for specified features,
            using JSON output format.
        3.  Verify the command's success or failure based on feature dependencies.
            - For successful connections: verify system is registered and feature states
            match expected values from JSON output.
            - For failed connections: verify return code 64 and system is not registered.
        4.  Verify yggdrasil service state based on remote-management feature.
    :expectedresults:
        1.  System is successfully disconnected (if previously connected).
        2.  The 'rhc connect' command executes.
        3.  Command execution follows dependency rules:
            - For successful connections: system is registered, feature states in JSON
            output match expected values.
            - For failed connections: system is not registered, return code is 64.
        4.  Yggdrasil service is active only when remote-management is enabled with
            all its dependencies satisfied, otherwise not active.
    """
    with contextlib.suppress(Exception):
        rhc.disconnect()

    command_args = prepare_args_for_connect(test_config, auth="basic", output_format="json")

    # Add --enable-feature flags for enabled features
    for feature in enabled_features:
        command_args.extend(["--enable-feature", feature])

    # Add --disable-feature flags for disabled features
    for feature in disabled_features:
        command_args.extend(["--disable-feature", feature])

    command = ["connect"] + command_args

    if should_fail:
        # Special cases that should fail: e.g. remote-management enabled when content or analytics feature disabled
        result = rhc.run(*command, check=False)

        # Verify command failed with return code 64
        assert result.returncode == 64, (
            f"Expected return code 64 for invalid feature combination, "
            f"but got {result.returncode}"
        )

        # Verify system is not registered
        assert not rhc.is_registered, (
            f"System should not be registered with enabled_features={enabled_features} "
            f"when disabled_features={disabled_features}"
        )

    else:
        # Valid feature combinations
        result = rhc.run(*command)

        # Verify the system is registered
        assert rhc.is_registered

        # Parse JSON output
        json_output = json.loads(result.stdout)
        features = json_output["features"]

        # Verify each feature's enabled status matches expected states
        for feature_key, expected_enabled in expected_states.items():
            actual_enabled = features[feature_key]["enabled"]
            assert actual_enabled is expected_enabled, (
                f"Feature '{feature_key}' should have enabled={expected_enabled}, "
                f"but got enabled={actual_enabled}"
            )

    # Yggdrasil service should be active only when remote-management is fully enabled
    if is_remote_management_fully_enabled(enabled_features):
        assert yggdrasil_service_is_active()
    else:
        assert not yggdrasil_service_is_active()


@pytest.mark.tier1
@pytest.mark.parametrize("flag", ["--enable-feature", "--disable-feature"],)
def test_connect_with_nonexistent_feature(external_candlepin, rhc, test_config, flag):
    """
    :id: 1ff23e4c-d3ab-436e-b90c-000958fd829f
    :title: Verify RHC connect fails with appropriate error for non-existent features
    :parametrized: yes
    :description:
        This test verifies that the 'rhc connect' command fails with an appropriate error
        when trying to enable or disable a feature that doesn't exist. Only features defined
        in the valid feature list should be accepted. Any other feature name should be rejected.
    :tags: Tier 1
    :steps:
        1.  Ensure the system is disconnected from RHC.
        2.  Run 'rhc connect' with --enable-feature or --disable-feature flag using
            a non-existent feature name.
        3.  Verify the command fails with a non-zero return code.
        4.  Verify the system is not registered.
        5.  Verify yggdrasil service is not active.
        6.  Verify error message indicates invalid feature and lists valid features.
    :expectedresults:
        1.  System is successfully disconnected (if previously connected).
        2.  The 'rhc connect' command fails.
        3.  Return code is non-zero (command failed).
        4.  System is not registered.
        5.  Yggdrasil service is not active.
        6.  Error message mentions the invalid feature and lists all valid features.
    """
    with contextlib.suppress(Exception):
        rhc.disconnect()

    nonexistent_feature = "nonexistent-feature"

    command_args = prepare_args_for_connect(
        test_config, auth="activation-key", output_format="json"
    )

    # Add the flag with non-existent feature
    command_args.extend([flag, nonexistent_feature])

    command = ["connect"] + command_args
    result = rhc.run(*command, check=False)

    # Verify command failed
    assert result.returncode != 0, (
        f"Command should fail when using {flag} with non-existent feature '{nonexistent_feature}'"
    )

    assert not rhc.is_registered
    assert not yggdrasil_service_is_active()

    # Verify error message contains expected text
    error_output = result.stderr + result.stdout
    # Build the expected feature list string from the centralized feature list
    feature_list_str = ",".join(ALL_FEATURES_CLI)
    expected_error = f'feature "{nonexistent_feature}": no such feature exists ({feature_list_str})'
    assert expected_error in error_output, (
        f"Expected error message not found in output: {error_output}"
    )
