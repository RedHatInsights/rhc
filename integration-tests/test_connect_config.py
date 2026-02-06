"""
:casecomponent: rhc
:requirement: RHSS-291300
:subsystemteam: rhel-sst-csi-client-tools
:caseautomation: Automated
:upstream: Yes
"""

import contextlib
import json
import logging
import pytest

from utils import (
    yggdrasil_service_is_active,
    prepare_args_for_connect,
)

logger = logging.getLogger(__name__)

# ============================================================================
# Feature Configuration File Tests For Connect Command
# ============================================================================
# These tests verify that RHC correctly reads feature settings from the
# drop-in configuration file (/etc/rhc/config.toml.d/01-features.toml) and
# that CLI flags properly override config file settings when using the connect command.
#
# Precedence order (highest to lowest):
# 1. CLI flags (--enable-feature / --disable-feature)
# 2. Drop-in config file values
# 3. Built-in defaults (all features enabled)
# ============================================================================

# Map CLI feature names to JSON feature names
FEATURE_MAPPING = {
    "content": "content",
    "analytics": "analytics",
    "remote-management": "remote_management",
}
# List of all valid features (JSON names)
ALL_FEATURES_JSON = list(FEATURE_MAPPING.values())

@pytest.mark.tier1
@pytest.mark.parametrize(
    "config_content,expected_states",
    [
        # All features enabled via config
        (
            'features = { "content" = true, "analytics" = true, "remote-management" = true }',
            {"content": True, "analytics": True, "remote_management": True},
        ),
        # All features disabled via config
        (
            'features = { "content" = false, "analytics" = false, "remote-management" = false }',
            {"content": False, "analytics": False, "remote_management": False},
        ),
        # Mixed: content and analytics enabled, remote-management disabled
        (
            'features = { "content" = true, "analytics" = true, "remote-management" = false }',
            {"content": True, "analytics": True, "remote_management": False},
        ),
        # Only remote-management disabled
        (
            'features = { "remote-management" = false }',
            {"content": True, "analytics": True, "remote_management": False},
        ),
    ],
)
def test_connect_with_features_config_file(
    external_candlepin, rhc, test_config, features_config_file,
    config_content, expected_states
):
    """
    :id: 7a1b2c3d-4e5f-6789-0abc-def012345678
    :title: Verify RHC connection uses feature settings from drop-in config file
    :parametrized: yes
    :description:
        This test verifies that when connecting to RHC without CLI feature flags,
        the feature settings are read from the drop-in config file
        (/etc/rhc/config.toml.d/01-features.toml). Feature dependencies are
        respected (disabling content cascades to analytics and remote-management).
    :tags: Tier 1
    :steps:
        1. Create the features drop-in config file with specified content.
        2. Ensure the system is disconnected from RHC.
        3. Run 'rhc connect' without any --enable-feature or --disable-feature flags.
        4. Verify feature states in JSON output match expected values.
        5. Verify yggdrasil service state based on remote-management.
    :expectedresults:
        1. Config file is created successfully.
        2. System is disconnected.
        3. Connect command succeeds.
        4. Feature states match config file settings.
        5. Yggdrasil active only when remote-management is enabled with dependencies.
    """
    # Create the features config file
    features_config_file.write(config_content)

    with contextlib.suppress(Exception):
        rhc.disconnect()

    # Run connect without any feature CLI flags
    command_args = prepare_args_for_connect(
        test_config, auth="activation-key", output_format="json"
    )
    command = ["connect"] + command_args
    result = rhc.run(*command)

    assert rhc.is_registered

    # Parse JSON output and verify feature states
    json_output = json.loads(result.stdout)
    features = json_output["features"]

    for feature_key, expected_enabled in expected_states.items():
        actual_enabled = features[feature_key]["enabled"]
        assert actual_enabled is expected_enabled, (
            f"Feature '{feature_key}' should have enabled={expected_enabled} "
            f"from config file, but got enabled={actual_enabled}"
        )

    # Verify yggdrasil service state
    if expected_states.get("remote_management", False):
        assert yggdrasil_service_is_active()
    else:
        assert not yggdrasil_service_is_active()


@pytest.mark.tier1
@pytest.mark.parametrize(
    "config_content,cli_enable,cli_disable,expected_states,expected_return_code",
    [
        # Config disables content, CLI enables it - should enable all (defaults)
        (
            'features = { "content" = false }',
            ["content"],
            [],
            {"content": True, "analytics": True, "remote_management": True},
            0,
        ),
        # Config enables all, CLI disables remote-management
        (
            'features = { "content" = true, "analytics" = true, "remote-management" = true }',
            [],
            ["remote-management"],
            {"content": True, "analytics": True, "remote_management": False},
            0,
        ),
        # Config disables analytics, CLI enables it
        (
            'features = { "content" = true, "analytics" = false }',
            ["analytics"],
            [],
            {"content": True, "analytics": True, "remote_management": True},
            0,
        ),
        # Config enables all, CLI disables content - dependency conflict (exit 64)
        (
            'features = { "content" = true, "analytics" = true, "remote-management" = true }',
            [],
            ["content"],
            None,  # No expected states - command should fail
            64,
        ),
        # Config enables remote-management, CLI disables analytics - dependency conflict (exit 64)
        (
            'features = { "content" = true, "analytics" = true, "remote-management" = true }',
            [],
            ["analytics"],
            None,  # No expected states - command should fail
            64,
        ),
        # Config disables remote-management, CLI enables it
        (
            'features = { "remote-management" = false }',
            ["remote-management"],
            [],
            {"content": True, "analytics": True, "remote_management": True},
            0,
        ),
    ],
)
def test_connect_cli_flags_override_config_file(
    external_candlepin, rhc, test_config, features_config_file,
    config_content, cli_enable, cli_disable, expected_states, expected_return_code
):
    """
    :id: 8b2c3d4e-5f6a-7890-1bcd-ef0123456789
    :title: Verify CLI feature flags override drop-in config file settings
    :parametrized: yes
    :description:
        This test verifies that CLI flags (--enable-feature/--disable-feature)
        take precedence over settings in the drop-in config file. The precedence
        order is: CLI flags > config file > built-in defaults.
    :tags: Tier 1
    :steps:
        1. Create the features drop-in config file with specified content.
        2. Ensure the system is disconnected from RHC.
        3. Run 'rhc connect' with CLI feature flags that override config.
        4. Verify feature states in JSON output match expected (CLI-overridden) values.
    :expectedresults:
        1. Config file is created.
        2. System is disconnected.
        3. Connect command succeeds.
        4. CLI flags correctly override config file settings.
    """
    # Create the features config file
    features_config_file.write(config_content)

    with contextlib.suppress(Exception):
        rhc.disconnect()

    command_args = prepare_args_for_connect(
        test_config, auth="activation-key", output_format="json"
    )

    # Add CLI flags to override config
    for feature in cli_enable:
        command_args.extend(["--enable-feature", feature])
    for feature in cli_disable:
        command_args.extend(["--disable-feature", feature])

    result = rhc.run("connect", *command_args, check=(expected_return_code == 0))

    assert result.returncode == expected_return_code, (
        f"Expected return code {expected_return_code}, but got {result.returncode}"
    )

    if expected_return_code != 0:
        # Dependency conflict: CLI disables a feature that config-enabled features depend on
        assert not rhc.is_registered, (
            "System should not be registered when CLI flag conflicts with config file"
        )
        return

    assert rhc.is_registered

    # Parse JSON output and verify feature states
    json_output = json.loads(result.stdout)
    features = json_output["features"]

    for feature_key, expected_enabled in expected_states.items():
        actual_enabled = features[feature_key]["enabled"]
        assert actual_enabled is expected_enabled, (
            f"Feature '{feature_key}' should have enabled={expected_enabled} "
            f"(CLI override), but got enabled={actual_enabled}"
        )

    # Verify yggdrasil service state
    if expected_states.get("remote_management", False):
        assert yggdrasil_service_is_active()
    else:
        assert not yggdrasil_service_is_active()


@pytest.mark.tier1
def test_connect_without_config_file_uses_defaults(
    external_candlepin, rhc, test_config, features_config_file
):
    """
    :id: 9c3d4e5f-6a7b-8901-2cde-f01234567890
    :title: Verify RHC uses built-in defaults when no config file exists
    :description:
        This test verifies that when no drop-in config file exists,
        RHC uses the built-in defaults (all features enabled) unless
        CLI flags specify otherwise.
    :tags: Tier 1
    :steps:
        1. Ensure the features config file does NOT exist.
        2. Ensure the system is disconnected from RHC.
        3. Run 'rhc connect' without any feature flags.
        4. Verify all features are enabled (default state).
        5. Verify yggdrasil service is active.
    :expectedresults:
        1. Config file does not exist.
        2. System is disconnected.
        3. Connect command succeeds.
        4. All features are enabled by default.
        5. Yggdrasil service is active.
    """
    # Ensure config file does not exist
    features_config_file.remove()

    with contextlib.suppress(Exception):
        rhc.disconnect()

    command_args = prepare_args_for_connect(
        test_config, auth="activation-key", output_format="json"
    )
    command = ["connect"] + command_args
    result = rhc.run(*command)

    assert rhc.is_registered

    # Parse JSON output and verify all features are enabled (defaults)
    json_output = json.loads(result.stdout)
    features = json_output["features"]

    for feature_name in ALL_FEATURES_JSON:
        assert features[feature_name]["enabled"] is True, (
            f"Feature '{feature_name}' should be enabled by default when no config file exists"
        )
        assert features[feature_name]["successful"] is True, (
            f"Feature '{feature_name}' should be successful"
        )

    assert yggdrasil_service_is_active()


@pytest.mark.tier1
@pytest.mark.parametrize(
    "config_content,expected_states",
    [
        # Only content specified as true - unspecified use defaults
        (
            'features = { "content" = true }',
            {"content": True, "analytics": True, "remote_management": True},
        ),
        # Only analytics specified as false - cascades to remote-management
        (
            'features = { "analytics" = false }',
            {"content": True, "analytics": False, "remote_management": False},
        ),
        # Only remote-management specified as false
        (
            'features = { "remote-management" = false }',
            {"content": True, "analytics": True, "remote_management": False},
        ),
        # Only content specified as false - cascades to all
        (
            'features = { "content" = false }',
            {"content": False, "analytics": False, "remote_management": False},
        ),
    ],
)
def test_connect_with_partial_config_file(
    external_candlepin, rhc, test_config, features_config_file,
    config_content, expected_states
):
    """
    :id: ad4e5f6a-7b8c-9012-3def-012345678901
    :title: Verify RHC handles partial feature configuration in config file
    :parametrized: yes
    :description:
        This test verifies that when only some features are specified in the
        config file, unspecified features use their built-in defaults.
        Feature dependencies are still respected.
    :tags: Tier 1
    :steps:
        1. Create config file with only some features specified.
        2. Ensure the system is disconnected from RHC.
        3. Run 'rhc connect' without CLI flags.
        4. Verify specified features use config values, others use defaults.
    :expectedresults:
        1. Partial config file is created.
        2. System is disconnected.
        3. Connect command succeeds.
        4. Specified features follow config, unspecified use defaults.
    """
    # Create the partial features config file
    features_config_file.write(config_content)

    with contextlib.suppress(Exception):
        rhc.disconnect()

    command_args = prepare_args_for_connect(
        test_config, auth="activation-key", output_format="json"
    )
    command = ["connect"] + command_args
    result = rhc.run(*command)

    assert rhc.is_registered

    # Parse JSON output and verify feature states
    json_output = json.loads(result.stdout)
    features = json_output["features"]

    for feature_key, expected_enabled in expected_states.items():
        actual_enabled = features[feature_key]["enabled"]
        assert actual_enabled is expected_enabled, (
            f"Feature '{feature_key}' should have enabled={expected_enabled}, "
            f"but got enabled={actual_enabled}"
        )

    # Verify yggdrasil service state
    if expected_states.get("remote_management", False):
        assert yggdrasil_service_is_active()
    else:
        assert not yggdrasil_service_is_active()


@pytest.mark.tier1
def test_connect_config_file_dependency_violation_cli_override(
    external_candlepin, rhc, test_config, features_config_file
):
    """
    :id: be5f6a7b-8c9d-0123-4ef0-123456789012
    :title: Verify CLI can resolve dependency violations from config file
    :description:
        This test verifies that if the config file contains settings that would
        violate dependencies (e.g., remote-management=true but content=false),
        CLI flags can resolve the conflict by enabling required dependencies.
    :tags: Tier 1
    :steps:
        1. Create config with content=false and remote-management=true.
        2. Verify that 'rhc connect' fails when content is disabled and remote-management is enabled.
        3. Run 'rhc connect' with --enable-feature content to resolve dependency.
        4. Verify connection succeeds.
        5. Verify all three features are enabled.
        6. Verify yggdrasil service is active.
    :expectedresults:
        1. Config file with potential conflict is created.
        2. Connect command fails with config file that would violate dependencies.
        3. CLI flag resolves the dependency.
        4. Connect command succeeds.
        5. All features are correctly enabled.
        6. Yggdrasil service is active.
    """
    # Create config that would violate dependencies without CLI override
    config_content = 'features = { "content" = false, "remote-management" = true }'
    features_config_file.write(config_content)

    with contextlib.suppress(Exception):
        rhc.disconnect()

    command_args = prepare_args_for_connect(
        test_config, auth="activation-key", output_format="json"
    )
    
    # Verify that the command fails with return code 64 when content is disabled and remote-management is enabled
    result = rhc.run("connect", *command_args, check=False)
    assert result.returncode == 64, (
        f"Expected return code 64, but got {result.returncode}"
    )
    assert not rhc.is_registered, "System should not be registered when content is disabled and remote-management is enabled"
    
    # Run connect with --enable-feature content to resolve dependency violation
    command_args.extend(["--enable-feature", "content"])

    command = ["connect"] + command_args
    result = rhc.run(*command)

    assert rhc.is_registered

    # Parse JSON output
    json_output = json.loads(result.stdout)
    features = json_output["features"]

    # All features should now be enabled
    assert features["content"]["enabled"] is True
    assert features["analytics"]["enabled"] is True
    assert features["remote_management"]["enabled"] is True

    assert yggdrasil_service_is_active()
