"""
:casecomponent: rhc
:requirement: RHSS-XXXXX
:subsystemteam: rhel-sst-csi-client-tools
:caseautomation: Automated
:upstream: Yes
"""

import pytest

from utils.constants import (
    EXIT_CODE_UNAVAILABLE,
    RHC_SERVER_SERVICE,
    RHC_SERVER_SOCKET,
    VARLINK_METHOD_COLLECTOR_LIST,
)
from utils.varlink import run_varlinkctl
from utils.systemctl import (
    is_unit_active,
    is_unit_enabled,
    start_service,
    stop_service,
    enable_and_start_socket,
    disable_and_stop_socket,
)


@pytest.fixture
def fd3_socket_setup():
    """
    Fixture to ensure rhc-server.socket is enabled for FD3 socket activation tests.
    Stops the service if running to test auto-boot behavior.
    """
    socket_was_enabled = is_unit_enabled(RHC_SERVER_SOCKET)

    if not socket_was_enabled:
        enable_and_start_socket(RHC_SERVER_SOCKET)

    if is_unit_active(RHC_SERVER_SERVICE):
        stop_service(RHC_SERVER_SERVICE)

    yield

    if not socket_was_enabled:
        disable_and_stop_socket(RHC_SERVER_SOCKET)


@pytest.fixture
def socket_disabled():
    """Ensure rhc-server.socket is disabled and rhc-server.service is stopped."""
    socket_was_enabled = is_unit_enabled(RHC_SERVER_SOCKET)
    service_was_active = is_unit_active(RHC_SERVER_SERVICE)

    if service_was_active:
        stop_service(RHC_SERVER_SERVICE)
    disable_and_stop_socket(RHC_SERVER_SOCKET)

    yield

    if socket_was_enabled:
        enable_and_start_socket(RHC_SERVER_SOCKET)
    if service_was_active:
        start_service(RHC_SERVER_SERVICE)


@pytest.fixture
def service_running():
    """Ensure rhc-server.socket is enabled and rhc-server.service is running."""
    socket_was_enabled = is_unit_enabled(RHC_SERVER_SOCKET)
    service_was_active = is_unit_active(RHC_SERVER_SERVICE)

    if not socket_was_enabled:
        enable_and_start_socket(RHC_SERVER_SOCKET)
    if not service_was_active:
        start_service(RHC_SERVER_SERVICE)

    yield

    if not service_was_active:
        stop_service(RHC_SERVER_SERVICE)
    if not socket_was_enabled:
        disable_and_stop_socket(RHC_SERVER_SOCKET)


@pytest.mark.tier2
def test_fd3_socket_activation(fd3_socket_setup):
    """
    :id: d1e2f3a4-b5c6-7890-def1-23456789abcd
    :title: Verify FD3 socket activation boots rhc-server on varlink call
    :reference: https://redhat.atlassian.net/browse/CCT-1756
    :description:
        Test that when rhc-server.service is not running but rhc-server.socket
        is enabled, making a varlink call triggers systemd socket activation,
        automatically boots the service, and returns a correct response.
    :tags: Tier 2
    :steps:
        1. Ensure rhc-server.service is stopped
        2. Ensure rhc-server.socket is enabled
        3. Verify service is not active before the call
        4. Make a varlink call (com.redhat.rhc.collector.List)
        5. Verify the service becomes active (auto-booted)
        6. Verify the response is correct and well-formed
    :expectedresults:
        1. Service is stopped successfully
        2. Socket is enabled
        3. Service is not active before varlink call
        4. Varlink call succeeds
        5. Service becomes active after the call
        6. Response contains the expected "collectors" field
    """
    assert not is_unit_active(
        RHC_SERVER_SERVICE
    ), f"Service {RHC_SERVER_SERVICE} should not be active before FD3 call"

    response = run_varlinkctl(VARLINK_METHOD_COLLECTOR_LIST)

    assert "collectors" in response, "Response should contain 'collectors' field"
    assert isinstance(response["collectors"], list), "'collectors' should be a list"

    assert is_unit_active(
        RHC_SERVER_SERVICE
    ), f"Service {RHC_SERVER_SERVICE} should be active after FD3 socket activation"


@pytest.mark.tier2
def test_socket_disabled_varlink_and_cli_fail(rhc, socket_disabled):
    """
    :id: e2f3a4b5-c6d7-8901-ef12-3456789abcde
    :title: Verify Varlink and CLI fail with actionable error when socket is disabled
    :description:
        Test that when rhc-server.socket is disabled, both varlinkctl and
        ``rhc collector list`` fail with an actionable error. The fixture
        restores prior socket/service state in teardown.
    :tags: Tier 2
    :steps:
        1. Stop rhc-server.service and disable rhc-server.socket
        2. Call com.redhat.rhc.collector.List via varlinkctl
        3. Run ``rhc collector list``
        4. Restore prior state in teardown
    :expectedresults:
        1. Socket is disabled and service is stopped
        2. Varlink call fails (non-zero exit)
        3. CLI fails with exit 69 and hints to restart rhc-server.socket
        4. Prior socket/service state is restored
    """
    assert not is_unit_active(RHC_SERVER_SOCKET)
    assert not is_unit_active(RHC_SERVER_SERVICE)

    varlink_result = run_varlinkctl(VARLINK_METHOD_COLLECTOR_LIST, check=False)
    assert varlink_result.returncode != 0

    cli_result = rhc.run("collector", "list", check=False)
    assert cli_result.returncode == EXIT_CODE_UNAVAILABLE
    assert "rhc-server.socket is not available" in cli_result.stderr
    assert "systemctl restart rhc-server.socket" in cli_result.stderr


@pytest.mark.tier2
def test_service_already_running_varlink_succeeds(service_running):
    """
    :id: f3a4b5c6-d7e8-9012-f123-456789abcdef
    :title: Verify Varlink works when rhc-server.service is already running
    :description:
        Test that when rhc-server.service is already active, a Varlink call
        succeeds and the service remains active afterwards.
    :tags: Tier 2
    :steps:
        1. Ensure rhc-server.socket is enabled
        2. Ensure rhc-server.service is already running
        3. Make a varlink call (com.redhat.rhc.collector.List)
        4. Verify the service is still active
    :expectedresults:
        1. Socket is enabled
        2. Service is active before the call
        3. Varlink call succeeds with a valid collectors array
        4. Service stays active after the call
    """
    assert is_unit_active(RHC_SERVER_SERVICE)

    response = run_varlinkctl(VARLINK_METHOD_COLLECTOR_LIST)

    assert "collectors" in response
    assert isinstance(response["collectors"], list)
    assert is_unit_active(RHC_SERVER_SERVICE)
