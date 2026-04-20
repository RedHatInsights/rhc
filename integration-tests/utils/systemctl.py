"""
Systemctl-related utility functions for integration tests.

This module provides helper functions for managing systemd services, sockets,
and units using systemctl commands.
"""

import subprocess


def is_service_active(service_name: str) -> bool:
    """
    Check if a systemd service is active.

    :param service_name: Name of the systemd service
    :return: True if the service is active, False otherwise
    """
    result = subprocess.run(
        ["systemctl", "is-active", service_name],
        capture_output=True,
    )
    return result.returncode == 0


def stop_service(service_name: str) -> None:
    """
    Stop a systemd service.

    :param service_name: Name of the systemd service to stop
    """
    subprocess.run(
        ["systemctl", "stop", service_name],
        check=True,
        capture_output=True,
    )


def is_socket_enabled(socket_name: str) -> bool:
    """
    Check if a systemd socket is enabled.

    :param socket_name: Name of the systemd socket
    :return: True if the socket is enabled, False otherwise
    """
    result = subprocess.run(
        ["systemctl", "is-enabled", socket_name],
        capture_output=True,
    )
    return result.returncode == 0


def enable_and_start_socket(socket_name: str) -> None:
    """
    Enable and start a systemd socket.

    :param socket_name: Name of the systemd socket
    """
    subprocess.run(
        ["systemctl", "enable", "--now", socket_name],
        check=True,
        capture_output=True,
    )


def disable_and_stop_socket(socket_name: str) -> None:
    """
    Disable and stop a systemd socket.

    :param socket_name: Name of the systemd socket
    """
    subprocess.run(
        ["systemctl", "disable", "--now", socket_name],
        capture_output=True,
    )
