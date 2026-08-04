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

from utils.systemctl import is_service_active

MINIMAL_COLLECTOR_ID = "com.redhat.minimal"
MINIMAL_COLLECTOR_NAME = "Minimal Host Inventory Collector"
MINIMAL_COLLECTOR_CONFIG = "/usr/lib/rhc/collectors/com.redhat.minimal.toml"
MINIMAL_COLLECTOR_TIMER = "rhc-collector-com.redhat.minimal.timer"
MINIMAL_COLLECTOR_SERVICE = "rhc-collector-com.redhat.minimal.service"


@pytest.fixture(scope="module")
def rhc_server_socket():
    """
    Fixture to ensure rhc-server.socket is enabled and running before collector CLI tests.
    """
    socket_name = "rhc-server.socket"

    was_active = is_service_active(socket_name)

    if not was_active:
        subprocess.run(
            ["systemctl", "enable", "--now", socket_name],
            check=True,
            capture_output=True,
        )

    yield

    if not was_active:
        subprocess.run(
            ["systemctl", "disable", "--now", socket_name],
            check=False,
            capture_output=True,
        )


pytestmark = pytest.mark.usefixtures("rhc_server_socket")


@pytest.fixture
def minimal_collector():
    """
    Fixture for the packaged com.redhat.minimal collector.
    """
    assert os.path.exists(
        MINIMAL_COLLECTOR_CONFIG
    ), f"Packaged collector config missing: {MINIMAL_COLLECTOR_CONFIG}"
    return {
        "id": MINIMAL_COLLECTOR_ID,
        "name": MINIMAL_COLLECTOR_NAME,
        "config_path": MINIMAL_COLLECTOR_CONFIG,
        "service_name": MINIMAL_COLLECTOR_SERVICE,
        "timer_name": MINIMAL_COLLECTOR_TIMER,
    }


@pytest.fixture
def minimal_timer_cache():
    """
    Fixture to create a temporary timer cache for the minimal collector.
    Restores any pre-existing cache on teardown.
    """
    timer_dir = "/var/cache/rhc/collectors"
    timer_cache_path = os.path.join(timer_dir, f"{MINIMAL_COLLECTOR_ID}.json")
    last_run_timestamp = int(time.time()) - 3600  # 1 hour ago
    cache_content = {
        "last_started": {"timestamp": last_run_timestamp - 30},
        "last_finished": {"timestamp": last_run_timestamp, "exit_code": 0},
    }

    previous_cache = None
    if os.path.exists(timer_cache_path):
        with open(timer_cache_path) as f:
            previous_cache = f.read()

    os.makedirs(timer_dir, exist_ok=True)
    with open(timer_cache_path, "w") as f:
        json.dump(cache_content, f)

    yield {"path": timer_cache_path, "last_run": last_run_timestamp}

    if previous_cache is not None:
        with open(timer_cache_path, "w") as f:
            f.write(previous_cache)
    elif os.path.exists(timer_cache_path):
        os.remove(timer_cache_path)


@pytest.fixture
def minimal_collector_timer(minimal_collector, minimal_timer_cache):
    """
    Fixture that enables the packaged minimal collector timer.
    Restores the previous enable/active state on teardown.
    """
    was_enabled = (
        subprocess.run(
            ["systemctl", "is-enabled", MINIMAL_COLLECTOR_TIMER],
            capture_output=True,
            text=True,
            check=False,
        ).returncode
        == 0
    )
    was_active = is_service_active(MINIMAL_COLLECTOR_TIMER)

    subprocess.run(
        ["systemctl", "enable", "--now", MINIMAL_COLLECTOR_TIMER],
        check=True,
        capture_output=True,
    )

    yield {
        **minimal_collector,
        "last_run": minimal_timer_cache["last_run"],
    }

    if was_active:
        subprocess.run(
            ["systemctl", "start", MINIMAL_COLLECTOR_TIMER],
            check=False,
            capture_output=True,
        )
    else:
        subprocess.run(
            ["systemctl", "stop", MINIMAL_COLLECTOR_TIMER],
            check=False,
            capture_output=True,
        )

    if was_enabled:
        subprocess.run(
            ["systemctl", "enable", MINIMAL_COLLECTOR_TIMER],
            check=False,
            capture_output=True,
        )
    else:
        subprocess.run(
            ["systemctl", "disable", MINIMAL_COLLECTOR_TIMER],
            check=False,
            capture_output=True,
        )


@pytest.mark.tier2
def test_collector_cli_list(rhc, minimal_collector):
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
    assert minimal_collector["id"] in result.stdout
    assert minimal_collector["name"] in result.stdout


@pytest.mark.tier2
@pytest.mark.skip(
    reason=(
        "Known issue: JSON output does not match human-readable output, "
        "tracked by RHEL-217910."
    )
)
def test_collector_cli_list_format_json(rhc, minimal_collector):
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
        (c for c in collectors if c.get("id") == minimal_collector["id"]), None
    )
    assert collector is not None, f"Collector {minimal_collector['id']} not found"
    assert collector == {
        "id": minimal_collector["id"],
        "name": minimal_collector["name"],
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
def test_collector_cli_info(rhc, minimal_collector, minimal_timer_cache):
    """
    :id: 4d5e6f70-8192-0123-def0-234567890123
    :title: Verify rhc collector info shows collector details
    :description:
        Test that 'rhc collector info COLLECTOR' prints human-readable details
        for the packaged minimal collector, including last-run from the timer
        cache and next-run from the enabled systemd timer.
    :tags: Tier 2
    :steps:
        1. Create a timer cache for the minimal collector
        2. Run 'rhc collector info' with the minimal collector ID
        3. Verify exit code and expected fields in the output
    :expectedresults:
        1. Timer cache is created
        2. Command succeeds with exit code 0
        3. Output contains name, feature, config path, service, timer, and a
           relative last-run time
    """
    result = rhc.run("collector", "info", minimal_collector["id"], check=False)

    assert result.returncode == 0
    assert f"Name:      {minimal_collector['name']}" in result.stdout
    assert "Feature:   analytics" in result.stdout
    assert f"Config:   {minimal_collector['config_path']}" in result.stdout
    assert f"Service:  {minimal_collector['service_name']}" in result.stdout
    assert f"Timer:    {minimal_collector['timer_name']}" in result.stdout
    assert "Last run:" in result.stdout
    assert "Next run:" in result.stdout


@pytest.mark.tier2
@pytest.mark.skip(
    reason=(
        "Known issue: JSON output does not match human-readable output, "
        "tracked by RHEL-217910."
    )
)
def test_collector_cli_info_format_json(rhc, minimal_collector_timer):
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
        minimal_collector_timer["id"],
        check=False,
    )

    assert result.returncode == 0
    info = json.loads(result.stdout)

    assert "next_run" in info
    assert isinstance(info["next_run"], int)
    assert info["next_run"] > 0
    expected = {
        "name": minimal_collector_timer["name"],
        "feature": "analytics",
        "last_run": minimal_collector_timer["last_run"],
        "config_path": minimal_collector_timer["config_path"],
        "service_name": minimal_collector_timer["service_name"],
        "timer_name": minimal_collector_timer["timer_name"],
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
def test_collector_cli_timers(rhc, minimal_collector):
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
    assert minimal_collector["id"] in result.stdout
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
def test_collector_cli_timers_format_json(rhc, minimal_collector_timer):
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
        (c for c in timers if c.get("id") == minimal_collector_timer["id"]), None
    )
    assert collector is not None, f"Collector {minimal_collector_timer['id']} not found"

    assert "next_run" in collector
    assert isinstance(collector["next_run"], int)
    assert collector["next_run"] > 0
    assert collector["id"] == minimal_collector_timer["id"]
    assert collector["last_run"] == minimal_collector_timer["last_run"]
