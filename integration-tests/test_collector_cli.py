"""
:casecomponent: rhc
:requirement: RHSS-XXXXX
:subsystemteam: rhel-sst-csi-client-tools
:caseautomation: Automated
:upstream: Yes
"""

import contextlib
import json
import os
import subprocess
import time

import pytest

from utils import prepare_args_for_connect, poll_until
from utils.systemctl import is_unit_active, is_unit_enabled
from utils.constants import (
    MINIMAL_COLLECTOR_ID,
    MINIMAL_COLLECTOR_NAME,
    MINIMAL_COLLECTOR_CONFIG_PATH,
    MINIMAL_SERVICE_UNIT,
    MINIMAL_TIMER_UNIT,
    RHC_COLLECTOR,
    RHC_TMP_DIR,
    TIMER_CACHE_DIR,
    EXIT_CODE_MOCK_MINIMAL_COLLECTOR_EXECUTABLE,
)

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
def test_collector_enable_now_triggers_immediate_run(
    rhc, minimal_collector_timer_disabled
):
    """
    :id: 1a1575b3-aa12-4e71-a84a-4af97741b9d0
    :title: Verify 'rhc collector enable --now' enables timer and triggers immediate run
    :description:
        Test that ``rhc collector enable --now`` both enables the systemd
        timer and immediately triggers a collection run of the
        com.redhat.minimal collector, evidenced by a freshly written timer
        cache file. The run itself may still fail at the upload stage in
        environments without Red Hat registration/network access; that
        failure is orthogonal to what this test verifies, since the timer
        cache is written before the upload is attempted.
    :tags: Tier 2
    :steps:
        1. Ensure the minimal collector timer starts disabled and remove any
           stale timer cache
        2. Run ``rhc collector enable --now com.redhat.minimal``
        3. Verify the timer is enabled via systemctl
        4. Verify a fresh timer cache file appears, proving an immediate run
           was triggered
        5. If the command exited non-zero, verify it was only because the
           triggered run itself failed (e.g. upload)
    :expectedresults:
        1. Timer is disabled and no stale cache exists
        2. ``rhc collector enable --now`` runs
        3. ``systemctl is-enabled`` reports the timer as enabled
        4. Timer cache file is created with a last_started timestamp at or
           after the command was issued
        5. Any non-zero exit code is explained by a known, allow-listed
           failure message
    """
    cache_path = os.path.join(TIMER_CACHE_DIR, f"{MINIMAL_COLLECTOR_ID}.json")
    if os.path.exists(cache_path):
        os.remove(cache_path)

    before = time.time()
    try:
        result = rhc.run(
            "collector", "enable", "--now", MINIMAL_COLLECTOR_ID, check=False
        )

        assert is_unit_enabled(MINIMAL_TIMER_UNIT), (
            "Timer should be enabled after 'rhc collector enable --now'"
        )

        assert poll_until(lambda: os.path.exists(cache_path), timeout_s=15), (
            "Expected an immediate collection run to write a timer cache file"
        )

        with open(cache_path) as f:
            cache = json.load(f)
        assert cache["last_started"]["timestamp"] >= before - 1, (
            "Timer cache should reflect a run triggered by 'enable --now', "
            f"got: {cache}"
        )

        if result.returncode != 0:
            # The timer was enabled and the run was triggered (both verified
            # above), so a non-zero exit here is only acceptable if it came
            # from the triggered run itself failing (e.g. no
            # registration/network for the upload step).
            combined_output = (result.stdout + result.stderr).lower()
            assert any(
                phrase in combined_output for phrase in ["failed to start service"]
            ), f"enable --now failed unexpectedly: {result.stderr}"
    finally:
        if os.path.exists(cache_path):
            os.remove(cache_path)


@pytest.mark.tier2
def test_collector_enable_without_now_does_not_trigger_run(
    rhc, minimal_collector_timer_disabled
):
    """
    :id: f802d8bc-bcc7-4d4c-b27b-2f05af0f5fe5
    :title: Verify 'rhc collector enable' without --now does not trigger a run
    :description:
        Test that ``rhc collector enable`` (without ``--now``) only enables
        the systemd timer and does not immediately trigger a collection run.
    :tags: Tier 2
    :steps:
        1. Ensure the minimal collector timer starts disabled and remove any
           stale timer cache
        2. Run ``rhc collector enable com.redhat.minimal``
        3. Verify the timer is enabled via systemctl
        4. Verify no timer cache file appears (no immediate run)
    :expectedresults:
        1. Timer is disabled and no stale cache exists
        2. ``rhc collector enable`` succeeds
        3. ``systemctl is-enabled`` reports the timer as enabled
        4. No timer cache file is created
    """
    cache_path = os.path.join(TIMER_CACHE_DIR, f"{MINIMAL_COLLECTOR_ID}.json")
    if os.path.exists(cache_path):
        os.remove(cache_path)

    result = rhc.run("collector", "enable", MINIMAL_COLLECTOR_ID, check=False)
    assert result.returncode == 0, f"enable failed: {result.stderr}"

    assert is_unit_enabled(MINIMAL_TIMER_UNIT), (
        "Timer should be enabled after 'rhc collector enable'"
    )

    assert not poll_until(lambda: os.path.exists(cache_path), timeout_s=3), (
        "'rhc collector enable' without --now should not trigger an immediate run"
    )


@pytest.mark.tier2
def test_collector_disable_now_stops_inflight_service(
    rhc, minimal_collector_slow_service
):
    """
    :id: ea40584f-c4a3-4489-ba6e-7a9c7f76166f
    :title: Verify 'rhc collector disable --now' stops an in-flight collector run
    :description:
        Test that when the com.redhat.minimal collector service is actively
        running (simulated via a long-running service override),
        ``rhc collector disable --now`` disables the timer and stops the
        in-flight service rather than letting it run to completion.
    :tags: Tier 2
    :steps:
        1. Override the collector service to run a long-lived command and
           start it directly to simulate an in-flight run
        2. Verify the service is active before disabling
        3. Run ``rhc collector disable --now com.redhat.minimal``
        4. Verify the timer is disabled and the service is no longer active
    :expectedresults:
        1. Service starts and is running
        2. Service is active
        3. ``rhc collector disable --now`` succeeds
        4. ``systemctl is-enabled`` reports disabled and the in-flight
           service is stopped promptly
    """
    subprocess.run(["systemctl", "start", MINIMAL_SERVICE_UNIT], check=True)
    assert poll_until(lambda: is_unit_active(MINIMAL_SERVICE_UNIT), timeout_s=10), (
        "Service should be active/in-flight before calling 'disable --now'"
    )

    result = rhc.run("collector", "disable", "--now", MINIMAL_COLLECTOR_ID, check=False)
    assert result.returncode == 0, f"disable --now failed: {result.stderr}"

    assert not is_unit_enabled(MINIMAL_TIMER_UNIT), (
        "Timer should be disabled after 'rhc collector disable --now'"
    )
    assert poll_until(lambda: not is_unit_active(MINIMAL_SERVICE_UNIT), timeout_s=10), (
        "In-flight service should be stopped by 'rhc collector disable --now'"
    )


@pytest.mark.tier2
def test_collector_disable_without_now_leaves_inflight_service_running(
    rhc, minimal_collector_slow_service
):
    """
    :id: 34f0495b-f5a6-4b40-96c6-eb963ef786ac
    :title: Verify 'rhc collector disable' without --now leaves an in-flight run alone
    :description:
        Test that when the com.redhat.minimal collector service is actively
        running (simulated via a long-running service override),
        ``rhc collector disable`` (without ``--now``) only disables the
        timer and leaves the in-flight service running.
    :tags: Tier 2
    :steps:
        1. Override the collector service to run a long-lived command and
           start it directly to simulate an in-flight run
        2. Verify the service is active before disabling
        3. Run ``rhc collector disable com.redhat.minimal``
        4. Verify the timer is disabled but the service is still active
    :expectedresults:
        1. Service starts and is running
        2. Service is active
        3. ``rhc collector disable`` succeeds
        4. ``systemctl is-enabled`` reports disabled and the in-flight
           service remains active
    """
    subprocess.run(["systemctl", "start", MINIMAL_SERVICE_UNIT], check=True)
    assert poll_until(lambda: is_unit_active(MINIMAL_SERVICE_UNIT), timeout_s=10), (
        "Service should be active/in-flight before calling 'disable'"
    )

    result = rhc.run("collector", "disable", MINIMAL_COLLECTOR_ID, check=False)
    assert result.returncode == 0, f"disable failed: {result.stderr}"

    assert not is_unit_enabled(MINIMAL_TIMER_UNIT), (
        "Timer should be disabled after 'rhc collector disable'"
    )
    assert poll_until(lambda: is_unit_active(MINIMAL_SERVICE_UNIT), timeout_s=3), (
        "'rhc collector disable' without --now should not stop an in-flight service"
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
    assert "varlink call failed: com.redhat.rhc.collector.NoSuchCollector" in result.stderr


@pytest.mark.tier2
def test_collector_cli_info_invalid_id(rhc):
    """
    :id: b2c3d4e5-f607-1829-3456-789012345678
    :title: Verify rhc collector info fails with invalid collector ID
    :description:
        Test that ``rhc collector info`` with an invalid collector ID fails.
    :tags: Tier 2
    :steps:
        1. Run ``rhc collector info`` with an invalid collector ID
        2. Verify the command fails
    :expectedresults:
        1. Command fails with non-zero exit code
        2. Error indicates the collector info lookup failed with an actionable error message
    """
    result = rhc.run("collector", "info", "invalid-collector-id", check=False)
    assert result.returncode != 0
    assert "failed to get collector info" in result.stderr
    assert "varlink call failed: com.redhat.rhc.collector.InvalidParameter" in result.stderr


@pytest.mark.tier2
def test_collector_cli_enable_missing_id(rhc):
    """
    :id: c2d3e4f5-g617-2839-4567-890123456789
    :title: Verify rhc collector enable fails for a missing collector ID
    :description:
        Test that ``rhc collector enable`` with a missing collector ID fails.
    :tags: Tier 2
    :steps:
        1. Run ``rhc collector enable`` with a missing collector ID
        2. Verify the command fails
    :expectedresults:
        1. Command fails with non-zero exit code
        2. Error indicates the collector enable failed with an actionable error message
    """
    result = rhc.run("collector", "enable", check=False)
    assert result.returncode != 0
    assert "rhc collector enable requires a collector ID" in result.stderr


@pytest.mark.tier2
def test_collector_cli_enable_nonexistent_id(rhc):
    """
    :id: e2f3g4h5-i637-4859-5678-901234567890
    :title: Verify rhc collector enable fails for a non-existent collector
    :description:
        Test that ``rhc collector enable`` with a non-existent collector ID fails.
    :tags: Tier 2
    :steps:
        1. Run ``rhc collector enable`` with a non-existent collector ID
        2. Verify the command fails
    :expectedresults:
        1. Command fails with non-zero exit code
        2. Error indicates the collector enable failed with an actionable error message
    """
    result = rhc.run("collector", "enable", "nonexistent.collector.id", check=False)
    assert result.returncode != 0
    assert "collector nonexistent.collector.id not found" in result.stderr


@pytest.mark.tier2
def test_collector_cli_enable_invalid_id(rhc):
    """
    :id: d2e3f4g5-h627-3849-4567-890123456789
    :title: Verify rhc collector enable fails with invalid collector ID
    :description:
        Test that ``rhc collector enable`` with an invalid collector ID fails.
    :tags: Tier 2
    :steps:
        1. Run ``rhc collector enable`` with an invalid collector ID
        2. Verify the command fails
    :expectedresults:
        1. Command fails with non-zero exit code
        2. Error indicates the collector enable failed with an actionable error message
    """
    result = rhc.run("collector", "enable", "invalid-collector-id", check=False)
    assert result.returncode != 0
    assert "collector invalid-collector-id not found" in result.stderr


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


@pytest.mark.tier2
def test_collector_cli_help(rhc):
    """
    :id: d4e5f6g7-h829-3045-6789-012345678901
    :title: Verify rhc collector help shows usage information
    :description:
        Test that ``rhc collector help`` shows usage information for the collector CLI.
    :tags: Tier 2
    :steps:
        1. Run ``rhc collector help``
        2. Verify the command succeeds and shows usage information
    :expectedresults:
        1. Command succeeds with exit code 0
        2. Output shows usage information for the collector CLI
    """
    result = rhc.run("collector", "help", check=False)
    assert result.returncode == 0
    out = result.stdout
    assert "NAME:" in out
    assert "rhc collector - Collect data for analysis" in out
    assert "USAGE:" in out
    assert "rhc collector COMMAND [command options]" in out
    assert "COMMANDS:" in out
    assert "info" in out and "Display collector information" in out
    assert "list" in out and "List available collectors" in out
    assert "timers" in out and "List collector timers" in out
    assert "enable" in out and "Enable timer-based collection" in out
    assert "disable" in out and "Disable timer-based collection" in out
    assert "OPTIONS:" in out
    assert "--help, -h" in out


@pytest.mark.tier1
def test_minimal_collector_upload_production(external_candlepin, rhc, test_config):
    """
    :id: 7c1e4a2b-9d3f-4e80-b1a6-0f2c8d9e5a11
    :title: Run com.redhat.minimal end-to-end against production Ingress
    :description:
        Register with production RHSM from the [prod] settings.toml section,
        run rhc-collector for the minimal collector and verify collection and
        the tarball archive upload both succeed. The tarball archive is uploaded
        to the production ingress url: https://cert.console.redhat.com/api/ingress/v1/upload.
    :tags: Tier 1
    :steps:
        1. Run rhc-collector run com.redhat.minimal
        2. Verify rhc-collector exits with exit code 0
        3. Verify a log for a successful archive upload
        4. Verify timer cache last_finished.exit_code is 0
    :expectedresults:
        1. Minimal collector collect succeeds with exit code 0
        2. rhc-collector exits with exit code 0
        3. Archive upload is successful
        4. Timer cache last_finished.exit_code is 0, indicating successful collection
    """
    if test_config.environment != "prod":
        pytest.skip("requires ENV_FOR_DYNACONF=prod (production RHSM and Ingress)")

    with contextlib.suppress(Exception):
        rhc.disconnect()

    command_args = prepare_args_for_connect(test_config, auth="activation-key")
    rhc.run("connect", *command_args)

    assert rhc.is_registered
    assert os.path.exists("/etc/pki/consumer/cert.pem")

    cache_path = os.path.join(TIMER_CACHE_DIR, f"{MINIMAL_COLLECTOR_ID}.json")
    if os.path.exists(cache_path):
        os.remove(cache_path)

    try:
        result = subprocess.run(
            [RHC_COLLECTOR, "run", MINIMAL_COLLECTOR_ID],
            capture_output=True,
            text=True,
        )

        combined_output = result.stdout + result.stderr
        assert result.returncode == 0, combined_output
        assert "collector has ran successfully" in combined_output
        assert "Successfully uploaded archive" in combined_output
        assert "status code" not in combined_output
        assert os.path.exists(cache_path)
        with open(cache_path) as cache_file:
            cache = json.load(cache_file)
        assert cache["last_finished"]["exit_code"] == 0
    finally:
        if os.path.exists(cache_path):
            os.remove(cache_path)


@pytest.mark.tier1
def test_minimal_collector_unregistered(rhc):
    """
    :id: 3901cf49-6fc1-478f-9dba-803eeb0e7857
    :title: Unregistered host, com.redhat.minimal fails with exit code non zero
    :description:
        Host is unregistered, com.redhat.minimal collect fails due to the missing
        consumer certificate at /etc/pki/consumer/cert.pem, rhc-collector writes the
        timer cache with last_finished.exit_code being non zero.
    :tags: Tier 1
    :steps:
        1. Unregister the host
        2. Run rhc-collector run com.redhat.minimal
        3. Verify minimal collector collect fails with exit code non zero
        4. Verify no archive upload is attempted
        5. Verify timer cache last_finished.exit_code is non zero, indicating failed collection
    :expectedresults:
        1. Host is unregistered with consumer certificate removed
        2. rhc-collector exits with exit code non zero
        3. Minimal collector collect fails with exit code non zero
        4. Archive upload is not successful
        5. Timer cache last_finished.exit_code is non zero, indicating failed collection
    """
    with contextlib.suppress(Exception):
        rhc.disconnect()
    assert not rhc.is_registered
    assert not os.path.exists("/etc/pki/consumer/cert.pem")

    cache_path = os.path.join(TIMER_CACHE_DIR, f"{MINIMAL_COLLECTOR_ID}.json")
    if os.path.exists(cache_path):
        os.remove(cache_path)

    try:
        result = subprocess.run(
            [RHC_COLLECTOR, "run", MINIMAL_COLLECTOR_ID],
            capture_output=True,
            text=True,
        )

        combined_output = result.stdout + result.stderr
        assert result.returncode == 1, combined_output
        assert "failed to execute collector" in combined_output
        assert "Successfully uploaded archive" not in combined_output
        assert os.path.exists(cache_path)
        with open(cache_path) as cache_file:
            cache = json.load(cache_file)
        assert cache["last_finished"]["exit_code"] != 0
    finally:
        if os.path.exists(cache_path):
            os.remove(cache_path)


@pytest.mark.tier1
def test_minimal_collector_executable_failure(failing_minimal_collector_executable):
    """
    :id: 9d2f5b3c-0e41-5f91-c2b7-1a3d9e6f6b22
    :title: com.redhat.minimal collect fails with non-zero exit_code
    :description:
        com.redhat.minimal collect fails due to the executable returning a non-zero
        exit code. rhc-collector writes the timer cache, and last_finished.exit_code
        with expected exit code.
    :tags: Tier 1
    :steps:
        1. Run rhc-collector run com.redhat.minimal
        2. Verify minimal collector collect fails with exit code non zero
        3. Verify no archive upload is attempted
        4. Verify timer cache last_finished.exit_code equals the expected exit code
    :expectedresults:
        1. rhc-collector exits non-zero
        2. Minimal collector collect fails with expected exit code
        3. Archive upload is not successful
        4. Timer cache last_finished.exit_code equals the expected exit code
    """
    cache_path = os.path.join(TIMER_CACHE_DIR, f"{MINIMAL_COLLECTOR_ID}.json")
    if os.path.exists(cache_path):
        os.remove(cache_path)

    try:
        result = subprocess.run(
            [RHC_COLLECTOR, "run", MINIMAL_COLLECTOR_ID],
            capture_output=True,
            text=True,
        )

        combined_output = result.stdout + result.stderr
        assert result.returncode == 1, combined_output
        assert "failed to execute collector" in combined_output
        assert "Successfully uploaded archive" not in combined_output
        assert os.path.exists(cache_path)
        with open(cache_path) as cache_file:
            cache = json.load(cache_file)
        assert cache["last_finished"]["exit_code"] == EXIT_CODE_MOCK_MINIMAL_COLLECTOR_EXECUTABLE
    finally:
        if os.path.exists(cache_path):
            os.remove(cache_path)


@pytest.mark.tier1
def test_minimal_collector_missing_executable(missing_minimal_collector_executable):
    """
    :id: 8d2f5b3c-0e41-5f91-c2b7-1a3d9e6f6b22
    :title: com.redhat.minimal executable missing and collect fails with non-zero exit code
    :description:
        com.redhat.minimal executable is missing, so collect fails.
        rhc-collector writes the timer cache, and last_finished.exit_code
        is non zero.
    :tags: Tier 1
    :steps:
        1. Run rhc-collector run com.redhat.minimal
        2. Verify no archive upload is attempted
        3. Verify timer cache last_finished.exit_code is non zero, indicating failed collection
    :expectedresults:
        1. rhc-collector exits with exit code non zero
        2. Archive upload is not successful
        3. Timer cache last_finished.exit_code is non zero, indicating failed collection
    """
    cache_path = os.path.join(TIMER_CACHE_DIR, f"{MINIMAL_COLLECTOR_ID}.json")
    if os.path.exists(cache_path):
        os.remove(cache_path)

    try:
        result = subprocess.run(
            [RHC_COLLECTOR, "run", MINIMAL_COLLECTOR_ID],
            capture_output=True,
            text=True,
        )

        combined_output = result.stdout + result.stderr
        assert result.returncode == 1, combined_output
        assert "failed to execute collector" in combined_output
        assert "Successfully uploaded archive" not in combined_output
        assert os.path.exists(cache_path)
        with open(cache_path) as cache_file:
            cache = json.load(cache_file)
        assert cache["last_finished"]["exit_code"] != 0
    finally:
        if os.path.exists(cache_path):
            os.remove(cache_path)
