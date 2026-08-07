"""
Systemctl-related utility functions for integration tests.

This module provides helper functions for managing systemd services, sockets,
and units using systemctl commands.
"""

import json
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


def is_unit_enabled(unit_name: str) -> bool:
    """
    Check if a systemd unit (timer, socket, service, etc.) is enabled.

    :param unit_name: Name of the systemd unit
    :return: True if the unit is enabled, False otherwise
    """
    result = subprocess.run(
        ["systemctl", "is-enabled", unit_name],
        capture_output=True,
        text=True,
    )
    return result.stdout.strip() == "enabled"


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


def get_timer_next_trigger(timer_unit: str):
    """
    Return the next trigger time for *timer_unit* as a Unix timestamp (int),
    or None if the timer is not scheduled.

    :param timer_unit: Name of the systemd timer unit
    :return: Unix timestamp (seconds) or None
    """
    result = subprocess.run(
        ["systemctl", "list-timers", timer_unit, "--all", "--output=json"],
        capture_output=True,
        text=True,
        check=True,
    )
    timers = json.loads(result.stdout)
    if not timers:
        return None
    next_us = timers[0].get("next")
    if next_us and next_us > 0:
        return next_us // 1_000_000
    return None
