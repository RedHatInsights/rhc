"""
:casecomponent: rhc
:requirement: RHSS-XXXXX
:subsystemteam: rhel-sst-csi-client-tools
:caseautomation: Automated
:upstream: Yes
"""

import pytest
import json
import subprocess
import os

from conftest import (
    RHC_COLLECTOR,
    TIMER_CACHE_DIR,
    MINIMAL_COLLECTOR_ID,
    MINIMAL_TIMER_UNIT,
)
from utils.systemctl import is_unit_enabled

pytestmark = pytest.mark.usefixtures("rhc_server_socket")


@pytest.mark.tier2
def test_rhc_collector_writes_timer_cache(collector_config):
    """
    :id: b3644f21-1f2c-429b-b0eb-686749213a6a
    :title: Verify rhc-collector writes timer cache after execution
    :description:
        Test that running rhc-collector creates a timer cache file with
        execution timing information.
    :tags: Tier 2
    :steps:
        1. Create test collector configuration
        2. Run rhc-collector with a simple command
        3. Verify timer cache file is created with expected fields
    :expectedresults:
        1. Test collector is created
        2. rhc-collector runs the command
        3. Cache file contains last_started and last_finished timestamps
    """
    collector_id = collector_config["id"]
    cache_path = os.path.join(TIMER_CACHE_DIR, f"{collector_id}.json")

    os.makedirs("/var/tmp/rhc", exist_ok=True)
    os.makedirs(TIMER_CACHE_DIR, exist_ok=True)
    if os.path.exists(cache_path):
        os.remove(cache_path)

    try:
        subprocess.run(
            [RHC_COLLECTOR, "run", collector_id],
            capture_output=True,
            check=False,
        )
        assert os.path.exists(cache_path), "Timer cache file should be created"

        with open(cache_path) as cache_file:
            cache = json.load(cache_file)

        assert "last_started" in cache
        assert "last_finished" in cache
        assert cache["last_finished"]["exit_code"] == 0
    finally:
        if os.path.exists(cache_path):
            os.remove(cache_path)


@pytest.mark.tier2
def test_collector_enable_disable_via_cli(minimal_collector_timer_disabled):
    """
    :id: d0e1f2a3-b4c5-6789-0abc-def012345678
    :title: Verify enable/disable via CLI updates systemd timer state
    :description:
        Test that ``rhc collector enable`` enables the systemd timer and
        ``rhc collector disable`` disables it again, using the shipped
        com.redhat.minimal collector.
    :tags: Tier 2
    :steps:
        1. Ensure the minimal collector timer starts disabled
        2. Run ``rhc collector enable com.redhat.minimal``
        3. Verify the timer is enabled via systemctl
        4. Run ``rhc collector disable com.redhat.minimal``
        5. Verify the timer is disabled via systemctl
    :expectedresults:
        1. Timer is initially disabled
        2. ``rhc collector enable`` succeeds
        3. ``systemctl is-enabled`` reports enabled
        4. ``rhc collector disable`` succeeds
        5. ``systemctl is-enabled`` reports disabled
    """
    assert not is_unit_enabled(MINIMAL_TIMER_UNIT), "Timer should start disabled"

    result = subprocess.run(
        ["rhc", "collector", "enable", MINIMAL_COLLECTOR_ID],
        capture_output=True,
        text=True,
    )
    assert result.returncode == 0, f"enable failed: {result.stderr}"
    assert is_unit_enabled(MINIMAL_TIMER_UNIT), (
        "Timer should be enabled after 'rhc collector enable'"
    )

    result = subprocess.run(
        ["rhc", "collector", "disable", MINIMAL_COLLECTOR_ID],
        capture_output=True,
        text=True,
    )
    assert result.returncode == 0, f"disable failed: {result.stderr}"
    assert not is_unit_enabled(MINIMAL_TIMER_UNIT), (
        "Timer should be disabled after 'rhc collector disable'"
    )


@pytest.mark.tier2
def test_collector_enable_missing_timer(collector_config):
    """
    :id: e1f2a3b4-c5d6-7890-1bcd-ef0123456789
    :title: Verify enable fails with actionable message when timer unit is missing
    :description:
        Test that ``rhc collector enable`` returns a non-zero exit code and
        an actionable error message when the systemd timer unit does not
        exist on disk.  Uses a test collector that has a config
        but no systemd units.
    :tags: Tier 2
    :steps:
        1. Create a collector config (no systemd timer/service units)
        2. Run ``rhc collector enable ID``
        3. Verify the command fails with non-zero exit code
        4. Verify output contains an actionable error about the missing timer
    :expectedresults:
        1. Collector config is created
        2. ``rhc collector enable`` fails
        3. Exit code is non-zero
        4. Output mentions the missing timer or failure to enable
    """
    collector_id = collector_config["id"]

    result = subprocess.run(
        ["rhc", "collector", "enable", collector_id],
        capture_output=True,
        text=True,
    )

    assert result.returncode != 0, "enable should fail when timer unit is missing"

    combined_output = result.stdout + result.stderr
    assert any(
        phrase in combined_output.lower()
        for phrase in [
            "does not exist",
            "failed to enable timer",
            "need to be installed",
        ]
    ), f"Expected actionable error about missing timer, got: {combined_output}"
