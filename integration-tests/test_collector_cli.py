"""
:casecomponent: rhc
:requirement: RHSS-XXXXX
:subsystemteam: rhel-sst-csi-client-tools
:caseautomation: Automated
:upstream: Yes
"""

import json
import os
import subprocess
import time

import pytest

from utils.constants import (
    MINIMAL_COLLECTOR_ID,
    MINIMAL_COLLECTOR_NAME,
    MINIMAL_COLLECTOR_CONFIG_PATH,
    MINIMAL_SERVICE_UNIT,
    MINIMAL_TIMER_UNIT,
    RHC_COLLECTOR,
    RHC_TMP_DIR,
    TIMER_CACHE_DIR,
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

    os.makedirs(RHC_TMP_DIR, exist_ok=True)
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
def test_collector_enable_disable_via_cli(rhc, minimal_collector_timer_disabled):
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

    result = rhc.run("collector", "enable", MINIMAL_COLLECTOR_ID, check=False)
    assert result.returncode == 0, f"enable failed: {result.stderr}"
    assert is_unit_enabled(MINIMAL_TIMER_UNIT), (
        "Timer should be enabled after 'rhc collector enable'"
    )

    result = rhc.run("collector", "disable", MINIMAL_COLLECTOR_ID, check=False)
    assert result.returncode == 0, f"disable failed: {result.stderr}"
    assert not is_unit_enabled(MINIMAL_TIMER_UNIT), (
        "Timer should be disabled after 'rhc collector disable'"
    )


@pytest.mark.tier2
def test_collector_enable_missing_timer(rhc, collector_config):
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

    result = rhc.run("collector", "enable", collector_id, check=False)

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


@pytest.mark.tier2
def test_collector_cli_list(rhc):
    """
    :id: 1a2b3c4d-5e6f-7890-abcd-ef1234567890
    :title: Verify rhc collector list shows available collectors
    :description:
        Test that 'rhc collector list' prints a table of collector IDs and names
        including the packaged minimal collector.
    :tags: Tier 2
    :steps:
        1. Run 'rhc collector list'
        2. Verify exit code and table headers
        3. Verify the minimal collector appears in the output
    :expectedresults:
        1. Command succeeds with exit code 0
        2. Output contains ID and NAME headers
        3. Output contains the minimal collector ID and name
    """
    result = rhc.run("collector", "list", check=False)

    assert result.returncode == 0
    assert "ID" in result.stdout
    assert "NAME" in result.stdout
    assert MINIMAL_COLLECTOR_ID in result.stdout
    assert MINIMAL_COLLECTOR_NAME in result.stdout


@pytest.mark.tier2
@pytest.mark.skip(
    reason=(
        "Known issue: JSON output does not match human-readable output, "
        "tracked by RHEL-217910."
    )
)
def test_collector_cli_list_format_json(rhc):
    """
    :id: 2b3c4d5e-6f70-8901-bcde-f12345678901
    :title: Verify rhc collector list --format json returns list fields
    :description:
        Test that 'rhc collector list --format json' prints a JSON array whose
        objects match table columns (ID, NAME).
    :tags: Tier 2
    :steps:
        1. Run 'rhc collector list --format json'
        2. Parse the JSON output
        3. Verify each entry only has id and name
    :expectedresults:
        1. Command succeeds with exit code 0
        2. Output is a valid JSON array
        3. Minimal collector entry has only id and name matching the human table
    :bug: https://redhat.atlassian.net/browse/RHEL-217910
    """
    result = rhc.run("collector", "list", "--format", "json", check=False)

    assert result.returncode == 0
    collectors = json.loads(result.stdout)
    assert isinstance(collectors, list)

    collector = next(
        (c for c in collectors if c.get("id") == MINIMAL_COLLECTOR_ID), None
    )
    assert collector is not None, f"Collector {MINIMAL_COLLECTOR_ID} not found"
    assert collector == {
        "id": MINIMAL_COLLECTOR_ID,
        "name": MINIMAL_COLLECTOR_NAME,
    }


@pytest.mark.tier2
def test_collector_cli_list_invalid_format(rhc):
    """
    :id: 3c4d5e6f-7081-9012-cdef-123456789012
    :title: Verify rhc collector list rejects unsupported format
    :description:
        Test that 'rhc collector list --format' with an unsupported value fails.
    :tags: Tier 2
    :steps:
        1. Run 'rhc collector list --format yaml'
        2. Verify the command fails with an unsupported format error
    :expectedresults:
        1. Command fails with non-zero exit code
        2. Error mentions unsupported format
    """
    result = rhc.run("collector", "list", "--format", "yaml", check=False)

    assert result.returncode != 0
    output = result.stdout + result.stderr
    assert "unsupported format" in output


@pytest.mark.tier2
def test_collector_cli_info(rhc, minimal_collector_timer_cache):
    """
    :id: 4d5e6f70-8192-0123-def0-234567890123
    :title: Verify rhc collector info shows collector details
    :description:
        Test that 'rhc collector info COLLECTOR' prints human-readable details
        for the packaged minimal collector
    :tags: Tier 2
    :steps:
        1. Create a timer cache for the minimal collector
        2. Run 'rhc collector info' with the minimal collector ID
        3. Verify exit code and expected fields in the output
    :expectedresults:
        1. Timer cache is created
        2. Command succeeds with exit code 0
        3. Output contains name, feature, config path, service, timer, and  last-run 
    """
    expected_last_run = minimal_collector_timer_cache["last_run"]
    last_run_stamp = time.strftime(
        "%Y-%m-%d %H:%M", time.localtime(expected_last_run)
    )

    result = rhc.run("collector", "info", MINIMAL_COLLECTOR_ID, check=False)

    assert result.returncode == 0
    assert "Name:" in result.stdout
    assert MINIMAL_COLLECTOR_NAME in result.stdout
    assert "Feature:" in result.stdout
    assert "analytics" in result.stdout
    assert "Config:" in result.stdout
    assert MINIMAL_COLLECTOR_CONFIG_PATH in result.stdout
    assert "Service:" in result.stdout
    assert MINIMAL_SERVICE_UNIT in result.stdout
    assert "Timer:" in result.stdout
    assert MINIMAL_TIMER_UNIT in result.stdout
    assert "Last run:" in result.stdout
    assert last_run_stamp in result.stdout
    assert "ago" in result.stdout
    assert "Next run:" in result.stdout


@pytest.mark.tier2
@pytest.mark.skip(
    reason=(
        "Known issue: JSON output does not match human-readable output, "
        "tracked by RHEL-217910."
    )
)
def test_collector_cli_info_format_json(rhc, minimal_collector_with_timing):
    """
    :id: 5e6f7081-9203-1234-ef01-345678901234
    :title: Verify rhc collector info --format json returns info fields
    :description:
        Test that 'rhc collector info --format json COLLECTOR' prints a JSON
        object whose fields match info output (name, feature,
        last/next run, config, service, timer).
    :tags: Tier 2
    :steps:
        1. Enable the minimal collector timer and create a timer cache
        2. Run 'rhc collector info --format json' with the minimal collector ID
        3. Parse the JSON and verify fields in info output
    :expectedresults:
        1. Timer and cache are set up
        2. Command succeeds with exit code 0
        3. Output is a JSON object with name, feature, last_run, next_run,
           config_path, service_name, and timer_name
    """
    result = rhc.run(
        "collector",
        "info",
        "--format",
        "json",
        MINIMAL_COLLECTOR_ID,
        check=False,
    )

    assert result.returncode == 0
    info = json.loads(result.stdout)

    assert "next_run" in info
    assert isinstance(info["next_run"], int)
    assert info["next_run"] > 0
    expected = {
        "name": MINIMAL_COLLECTOR_NAME,
        "feature": "analytics",
        "last_run": minimal_collector_with_timing["last_run"],
        "config_path": MINIMAL_COLLECTOR_CONFIG_PATH,
        "service_name": MINIMAL_SERVICE_UNIT,
        "timer_name": MINIMAL_TIMER_UNIT,
    }
    assert {k: v for k, v in info.items() if k != "next_run"} == expected


@pytest.mark.tier2
def test_collector_cli_info_missing_id(rhc):
    """
    :id: 8192a3b4-c536-4567-1234-678901234567
    :title: Verify rhc collector info requires a collector ID
    :description:
        Test that 'rhc collector info' without a collector ID fails with a
        usage error.
    :tags: Tier 2
    :steps:
        1. Run 'rhc collector info' without arguments
        2. Verify the command fails
    :expectedresults:
        1. Command fails with non-zero exit code
        2. Error indicates a collector ID is required
    """
    result = rhc.run("collector", "info", check=False)

    assert result.returncode != 0
    assert "requires a collector ID" in result.stderr


@pytest.mark.tier2
def test_collector_cli_info_nonexistent_id(rhc):
    """
    :id: 92a3b4c5-d647-5678-2345-789012345678
    :title: Verify rhc collector info fails for a non-existent collector
    :description:
        Test that 'rhc collector info' with an unknown collector ID fails.
    :tags: Tier 2
    :steps:
        1. Run 'rhc collector info' with a non-existent collector ID
        2. Verify the command fails
    :expectedresults:
        1. Command fails with non-zero exit code
        2. Error indicates the collector info lookup failed
    """
    result = rhc.run("collector", "info", "nonexistent.collector.id", check=False)

    assert result.returncode != 0
    assert "failed to get collector info" in result.stderr


@pytest.mark.tier2
def test_collector_cli_timers(rhc):
    """
    :id: a3b4c5d6-e758-6789-3456-890123456789
    :title: Verify rhc collector timers shows timer status table
    :description:
        Test that 'rhc collector timers' prints a table of collector timer
        information and a hint to use the info command.
    :tags: Tier 2
    :steps:
        1. Run 'rhc collector timers'
        2. Verify exit code, table headers, collector ID, and hint
    :expectedresults:
        1. Command succeeds with exit code 0
        2. Output contains ID, LAST, NEXT headers, the minimal collector ID,
           and the hint
    """
    result = rhc.run("collector", "timers", check=False)

    assert result.returncode == 0
    assert "ID" in result.stdout
    assert "LAST" in result.stdout
    assert "NEXT" in result.stdout
    assert MINIMAL_COLLECTOR_ID in result.stdout
    assert "Hint: Run 'rhc collector info COLLECTOR' to show more details." in (
        result.stdout
    )


@pytest.mark.tier2
@pytest.mark.skip(
    reason=(
        "Known issue: JSON output does not match human-readable output, "
        "tracked by RHEL-217910."
    )
)
def test_collector_cli_timers_format_json(rhc, minimal_collector_with_timing):
    """
    :id: b4c5d6e7-f869-7890-4567-901234567890
    :title: Verify rhc collector timers --format json returns timing fields
    :description:
        Test that 'rhc collector timers --format json' prints a JSON array
    :tags: Tier 2
    :steps:
        1. Enable the minimal collector timer and create a timer cache
        2. Run 'rhc collector timers --format json'
        3. Parse the JSON and verify each entry - id, last_run, next_run
    :expectedresults:
        1. Timer and cache are set up
        2. Command succeeds with exit code 0
        3. Output is a JSON array of objects with id/last_run/next_run
    """
    result = rhc.run("collector", "timers", "--format", "json", check=False)

    assert result.returncode == 0
    timers = json.loads(result.stdout)
    assert isinstance(timers, list)

    collector = next(
        (c for c in timers if c.get("id") == MINIMAL_COLLECTOR_ID), None
    )
    assert collector is not None, f"Collector {MINIMAL_COLLECTOR_ID} not found"

    assert "next_run" in collector
    assert isinstance(collector["next_run"], int)
    assert collector["next_run"] > 0
    assert collector["id"] == MINIMAL_COLLECTOR_ID
    assert collector["last_run"] == minimal_collector_with_timing["last_run"]
