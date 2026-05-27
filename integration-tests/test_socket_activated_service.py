"""
:casecomponent: rhc
:requirement: RHSS-XXXXX
:subsystemteam: rhel-sst-csi-client-tools
:caseautomation: Automated
:upstream: Yes
"""

import pytest

from utils.varlink import run_varlinkctl
from utils.systemctl import (
    is_service_active,
    is_socket_enabled,
    stop_service,
    enable_and_start_socket,
    disable_and_stop_socket,
)


@pytest.mark.tier1
def test_fd3_socket_activation(fd3_socket_setup):
    """
    :id: d1e2f3a4-b5c6-7890-def1-23456789abcd
    :title: Verify FD3 socket activation boots rhc-server on varlink call
    :reference: https://redhat.atlassian.net/browse/CCT-1756
    :description:
        Test that when rhc-server.service is not running but rhc-server.socket
        is enabled, making a varlink call triggers systemd socket activation,
        automatically boots the service, and returns a correct response.
    :tags: Tier 1
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
    service_name = fd3_socket_setup["service_name"]

    assert not is_service_active(
        service_name
    ), f"Service {service_name} should not be active before FD3 call"

    response = run_varlinkctl("com.redhat.rhc.collector.List")

    assert "collectors" in response, "Response should contain 'collectors' field"
    assert isinstance(response["collectors"], list), "'collectors' should be a list"

    assert is_service_active(
        service_name
    ), f"Service {service_name} should be active after FD3 socket activation"
