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
import time
import textwrap

from utils.varlink import run_varlinkctl
from utils.systemctl import is_service_active

RHC_COLLECTOR = "/usr/libexec/rhc/rhc-collector"
TIMER_CACHE_DIR = "/var/cache/rhc/collectors"


@pytest.fixture(scope="module")
def rhc_server_socket():
    """
    Fixture to ensure rhc-server.socket is enabled and running before collector tests.
    This is required for varlinkctl to communicate with the rhc-server.
    """
    socket_name = "rhc-server.socket"

    # Check if socket is already active
    was_active = is_service_active(socket_name)

    if not was_active:
        # Enable and start the socket
        subprocess.run(
            ["systemctl", "enable", "--now", socket_name],
            check=True,
            capture_output=True,
        )

    yield

    # Cleanup: restore original state
    if not was_active:
        subprocess.run(
            ["systemctl", "disable", "--now", socket_name],
            check=False,
            capture_output=True,
        )


# Ensure rhc_server_socket fixture is used for all tests in this module
pytestmark = pytest.mark.usefixtures("rhc_server_socket")


@pytest.fixture
def collector_config():
    """
    Fixture to create a test collector configuration.
    """
    collector_dir = "/usr/lib/rhc/collectors"
    collector_id = "test.integration.collector"
    collector_config_path = os.path.join(collector_dir, f"{collector_id}.toml")

    config_content = textwrap.dedent("""
        [meta]
        name = "Test Integration Collector"
        feature = "analytics"
        type = "ingress"

        [ingress]
        user = "root"
        group = "root"
        content_type = "application/vnd.redhat.test.collection"
    """).strip()

    # Create the collector config
    os.makedirs(collector_dir, exist_ok=True)
    with open(collector_config_path, "w") as f:
        f.write(config_content)

    yield {
        "id": collector_id,
        "name": "Test Integration Collector",
        "config_path": collector_config_path,
    }

    # Cleanup
    if os.path.exists(collector_config_path):
        os.remove(collector_config_path)


@pytest.fixture
def collector_timer_cache(collector_config):
    """
    Fixture to create timer cache for the test collector.
    """
    timer_dir = "/var/cache/rhc/collectors"
    collector_id = collector_config["id"]
    timer_cache_path = os.path.join(timer_dir, f"{collector_id}.json")

    # Create a timer cache with last run timestamp
    last_run_timestamp = int(time.time()) - 3600  # 1 hour ago
    cache_content = {
        "last_started": {"timestamp": last_run_timestamp - 30},
        "last_finished": {"timestamp": last_run_timestamp, "exit_code": 0},
    }

    os.makedirs(timer_dir, exist_ok=True)
    with open(timer_cache_path, "w") as f:
        json.dump(cache_content, f)

    yield {"path": timer_cache_path, "last_run": last_run_timestamp}

    # Cleanup
    if os.path.exists(timer_cache_path):
        os.remove(timer_cache_path)


@pytest.mark.tier2
def test_collector_list_method(collector_config):
    """
    :id: a1b2c3d4-e5f6-7890-abcd-ef1234567890
    :title: Verify collector List method returns all collectors
    :description:
        Test that the com.redhat.rhc.collector.List method returns
        a list of all available collectors with their details.
    :tags: Tier 2
    :steps:
        1. Call com.redhat.rhc.collector.List via varlinkctl
        2. Verify the response structure
        3. Verify collectors array is returned
        4. Verify each collector has required fields
    :expectedresults:
        1. The varlink call succeeds
        2. Response contains "collectors" key
        3. Collectors array contains CollectorInfo objects
        4. Each collector has id, name, config_path, service_name, timer_name
    """
    response = run_varlinkctl("com.redhat.rhc.collector.List")

    # Verify response structure
    assert "collectors" in response
    assert isinstance(response["collectors"], list)

    # If collectors exist, verify their structure
    if len(response["collectors"]) > 0:
        for collector in response["collectors"]:
            # Required fields
            assert "id" in collector
            assert "name" in collector
            assert "config_path" in collector
            assert "service_name" in collector
            assert "timer_name" in collector

            # Check types
            assert isinstance(collector["id"], str)
            assert isinstance(collector["name"], str)
            assert isinstance(collector["config_path"], str)
            assert isinstance(collector["service_name"], str)
            assert isinstance(collector["timer_name"], str)

            # Optional fields
            if "feature" in collector:
                assert collector["feature"] is None or isinstance(
                    collector["feature"], str
                )
            if "last_run" in collector:
                assert isinstance(collector["last_run"], int)
            if "next_run" in collector:
                assert isinstance(collector["next_run"], int)


@pytest.mark.tier2
def test_collector_info_method_with_test_collector(collector_config):
    """
    :id: b2c3d4e5-f6a7-8901-bcde-f12345678901
    :title: Verify collector Info method returns details for a specific collector
    :description:
        Test that the com.redhat.rhc.collector.Info method returns
        detailed information for a specific collector.
    :tags: Tier 2
    :steps:
        1. Create a test collector configuration
        2. Call com.redhat.rhc.collector.Info with the test collector ID
        3. Verify the response structure
        4. Verify collector details match configuration
    :expectedresults:
        1. Test collector is created successfully
        2. The varlink call succeeds
        3. Response contains "info" key with CollectorInfo
        4. Collector details match the test configuration
    """
    collector_id = collector_config["id"]

    response = run_varlinkctl("com.redhat.rhc.collector.Info", {"id": collector_id})

    # Verify response structure
    assert "info" in response
    info = response["info"]

    # Verify required fields match
    assert info["id"] == collector_id
    assert info["name"] == collector_config["name"]
    assert info["config_path"] == collector_config["config_path"]
    assert info["service_name"] == f"rhc-collector-{collector_id}.service"
    assert info["timer_name"] == f"rhc-collector-{collector_id}.timer"

    # Verify optional fields
    assert info.get("feature") == "analytics"


@pytest.mark.tier2
def test_collector_info_with_timer_cache(
    collector_config, collector_timer_cache
):
    """
    :id: c3d4e5f6-a7b8-9012-cdef-123456789012
    :title: Verify collector Info includes timing information from cache
    :description:
        Test that the Info method returns last_run timestamp when
        timer cache exists for the collector.
    :tags: Tier 2
    :steps:
        1. Create test collector configuration
        2. Create timer cache with last run information
        3. Call com.redhat.rhc.collector.Info
        4. Verify last_run field is present and correct
    :expectedresults:
        1. Test collector and cache are created
        2. The varlink call succeeds
        3. Response includes last_run field
        4. last_run timestamp matches cache value
    """
    collector_id = collector_config["id"]
    expected_last_run = collector_timer_cache["last_run"]

    response = run_varlinkctl("com.redhat.rhc.collector.Info", {"id": collector_id})

    info = response["info"]

    # Verify last_run is present and matches
    assert "last_run" in info
    assert info["last_run"] == expected_last_run


@pytest.mark.tier2
def test_rhc_collector_writes_timer_cache(collector_config, rhc):
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
            [RHC_COLLECTOR, collector_id, "/bin/true"],
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
def test_collector_info_nonexistent_id():
    """
    :id: d4e5f6a7-b8c9-0123-def0-123456789abc
    :title: Verify collector Info returns error for non-existent collector ID
    :description:
        Test that the Info method returns NoSuchCollector error when
        called with a valid but non-existent collector ID.
    :tags: Tier 2
    :steps:
        1. Call com.redhat.rhc.collector.Info with non-existent collector ID
        2. Verify the call fails with NoSuchCollector error
    :expectedresults:
        1. The varlink call fails
        2. Error indicates NoSuchCollector
    """
    nonexistent_id = "nonexistent.collector.id"

    result = run_varlinkctl(
        "com.redhat.rhc.collector.Info", {"id": nonexistent_id}, check=False
    )

    # Should fail with non-zero exit code
    assert result.returncode != 0
    # Error output should mention NoSuchCollector
    assert "NoSuchCollector" in result.stderr


@pytest.mark.tier2
def test_collector_info_malformed_id():
    """
    :id: e5f6a7b8-c9d0-1234-ef01-23456789abcd
    :title: Verify collector Info returns error for malformed collector IDs
    :description:
        Test that the Info method returns InvalidParameter error when
        called with malformed collector IDs (invalid format).
    :tags: Tier 2
    :steps:
        1. Call com.redhat.rhc.collector.Info with various malformed IDs
        2. Verify each call fails with InvalidParameter error
    :expectedresults:
        1. All varlink calls fail
        2. Errors indicate InvalidParameter
    """
    malformed_ids = [
        "",  # Empty string
        "test",  # No dots
        "test_collector",  # Underscores not allowed
        "TEST.COLLECTOR",  # Uppercase not allowed
        "123",  # Just numbers
        ".test.collector",  # Leading dot
        "test.collector.",  # Trailing dot
        "test..collector",  # Double dots
    ]

    for malformed_id in malformed_ids:
        result = run_varlinkctl(
            "com.redhat.rhc.collector.Info", {"id": malformed_id}, check=False
        )

        # Should fail with non-zero exit code
        assert result.returncode != 0, f"Expected failure for ID: {malformed_id}"
        # Error output should mention InvalidParameter
        assert (
            "InvalidParameter" in result.stderr
        ), f"Expected InvalidParameter for ID: {malformed_id}, got: {result.stderr}"


@pytest.mark.tier2
def test_collector_list_includes_test_collector(collector_config):
    """
    :id: e5f6a7b8-c9d0-1234-ef01-23456789abcd
    :title: Verify List method includes newly added collector
    :description:
        Test that when a new collector configuration is added,
        it appears in the List method output.
    :tags: Tier 2
    :steps:
        1. Create a test collector configuration
        2. Call com.redhat.rhc.collector.List
        3. Verify test collector appears in the list
        4. Verify test collector has correct details
    :expectedresults:
        1. Test collector is created
        2. List call succeeds
        3. Test collector is in the returned list
        4. Details match the configuration
    """
    collector_id = collector_config["id"]

    response = run_varlinkctl("com.redhat.rhc.collector.List")

    collectors = response["collectors"]

    # Find our test collector in the list
    test_collector = None
    for collector in collectors:
        if collector["id"] == collector_id:
            test_collector = collector
            break

    # Verify test collector was found
    assert (
        test_collector is not None
    ), f"Test collector {collector_id} not found in list"

    # Verify details
    assert test_collector["name"] == collector_config["name"]
    assert test_collector["config_path"] == collector_config["config_path"]
    assert test_collector.get("feature") == "analytics"


@pytest.fixture
def collector_with_timing():
    """
    Fixture to create a fully-featured collector with config, cache, and systemd units.
    """
    collector_dir = "/usr/lib/rhc/collectors"
    cache_dir = "/var/cache/rhc/collectors"
    systemd_dir = "/etc/systemd/system"

    os.makedirs(collector_dir, exist_ok=True)
    os.makedirs(cache_dir, exist_ok=True)

    collector_id = "test.collector"
    collector_name = "Red Hat Lightspeed Advisor"

    # Create collector config
    config_path = os.path.join(collector_dir, f"{collector_id}.toml")
    config_content = textwrap.dedent("""
        [meta]
        name = "Red Hat Lightspeed Advisor"
        feature = "analytics"
        type = "ingress"

        [ingress]
        user = "root"
        group = "root"
        content_type = "application/vnd.redhat.advisor.collection"
    """).strip()

    with open(config_path, "w") as f:
        f.write(config_content)

    # Create timer cache
    cache_path = os.path.join(cache_dir, f"{collector_id}.json")
    last_finished_timestamp = int(time.time()) - 3600  # 1 hour ago
    last_started_timestamp = last_finished_timestamp - 30  # Started 30 seconds before finish

    cache_content = {
        "last_started": {"timestamp": last_started_timestamp},
        "last_finished": {"timestamp": last_finished_timestamp, "exit_code": 0},
    }

    with open(cache_path, "w") as f:
        json.dump(cache_content, f)

    # Create systemd timer and service
    timer_path = os.path.join(systemd_dir, f"rhc-collector-{collector_id}.timer")
    service_path = os.path.join(systemd_dir, f"rhc-collector-{collector_id}.service")

    timer_content = textwrap.dedent("""
        [Unit]
        Description=RHC Test Collector Timer
        Documentation=https://github.com/RedHatInsights/rhc

        [Timer]
        OnCalendar=hourly
        Persistent=true

        [Install]
        WantedBy=timers.target
    """).strip()

    service_content = textwrap.dedent("""
        [Unit]
        Description=RHC Test Collector Service
        Documentation=https://github.com/RedHatInsights/rhc

        [Service]
        Type=oneshot
        ExecStart=/bin/true
    """).strip()

    with open(timer_path, "w") as f:
        f.write(timer_content)

    with open(service_path, "w") as f:
        f.write(service_content)

    # Enable and start the timer
    subprocess.run(["systemctl", "daemon-reload"], check=True)
    subprocess.run(
        ["systemctl", "enable", "--now", f"rhc-collector-{collector_id}.timer"],
        check=True,
    )

    yield {
        "id": collector_id,
        "name": collector_name,
        "config_path": config_path,
        "cache_path": cache_path,
        "timer_path": timer_path,
        "service_path": service_path,
        "last_run": last_finished_timestamp,
    }

    # Cleanup
    subprocess.run(
        ["systemctl", "disable", "--now", f"rhc-collector-{collector_id}.timer"],
        check=False,
    )

    for path in [timer_path, service_path]:
        if os.path.exists(path):
            os.remove(path)

    subprocess.run(["systemctl", "daemon-reload"], check=True)

    if os.path.exists(config_path):
        os.remove(config_path)

    if os.path.exists(cache_path):
        os.remove(cache_path)


@pytest.fixture
def collector_minimal():
    """
    Fixture to create a minimal collector with only a config file.
    """
    collector_dir = "/usr/lib/rhc/collectors"
    os.makedirs(collector_dir, exist_ok=True)

    collector_id = "test.collector1"
    collector_name = "Red Hat Lightspeed Advisor"

    config_path = os.path.join(collector_dir, f"{collector_id}.toml")
    config_content = textwrap.dedent("""
        [meta]
        name = "Red Hat Lightspeed Advisor"
        feature = "analytics"
        type = "ingress"

        [ingress]
        user = "root"
        group = "root"
        content_type = "application/vnd.redhat.advisor.collection"
    """).strip()

    with open(config_path, "w") as f:
        f.write(config_content)

    yield {
        "id": collector_id,
        "name": collector_name,
        "config_path": config_path,
    }

    # Cleanup
    if os.path.exists(config_path):
        os.remove(config_path)


@pytest.fixture
def multiple_test_collectors(collector_with_timing, collector_minimal):
    """
    Fixture that combines a fully-featured collector and a minimal collector.
    Simulates real-world scenarios with varied collector configurations.
    """
    return {
        "collector1": collector_with_timing,
        "collector2": collector_minimal,
    }


@pytest.mark.tier2
def test_collector_list_with_multiple_collectors(multiple_test_collectors):
    """
    :id: f6a7b8c9-d0e1-2345-f012-3456789abcde
    :title: Verify List method with multiple collectors and varied configurations
    :description:
        Test the com.redhat.rhc.collector.List method with multiple collectors
        where one has cache/timer data and one does not, simulating real-world scenarios.
    :tags: Tier 2
    :steps:
        1. Create two test collectors
        2. Create cache and systemd timer/service for first collector
        3. Enable and start the timer
        4. Call com.redhat.rhc.collector.List
        5. Verify both collectors appear in the list
        6. Verify first collector has last_run and next_run fields
        7. Verify second collector does not have last_run or next_run
    :expectedresults:
        1. Both collectors are created
        2. Timer is enabled and started successfully
        3. List call succeeds
        4. Both collectors appear in the list
        5. Collector with cache shows last_run timestamp
        6. Collector with timer shows next_run timestamp
        7. Collector without cache/timer has no timing fields
    """
    collector1_info = multiple_test_collectors["collector1"]
    collector2_info = multiple_test_collectors["collector2"]

    response = run_varlinkctl("com.redhat.rhc.collector.List")

    assert "collectors" in response
    collectors = response["collectors"]

    # Find both collectors in the list
    collector1_data = None
    collector2_data = None

    for collector in collectors:
        if collector["id"] == collector1_info["id"]:
            collector1_data = collector
        elif collector["id"] == collector2_info["id"]:
            collector2_data = collector

    # Verify both collectors were found
    assert collector1_data is not None, f"Collector {collector1_info['id']} not found"
    assert collector2_data is not None, f"Collector {collector2_info['id']} not found"

    # Verify collector1 (with cache and timer)
    assert collector1_data["name"] == collector1_info["name"]
    assert collector1_data["config_path"] == collector1_info["config_path"]
    assert collector1_data["feature"] == "analytics"
    assert (
        collector1_data["service_name"]
        == f"rhc-collector-{collector1_info['id']}.service"
    )
    assert (
        collector1_data["timer_name"] == f"rhc-collector-{collector1_info['id']}.timer"
    )

    # Verify last_run is present and matches cache
    assert "last_run" in collector1_data
    assert collector1_data["last_run"] == collector1_info["last_run"]

    # Verify next_run is present (timer is enabled)
    assert "next_run" in collector1_data
    assert isinstance(collector1_data["next_run"], int)
    assert collector1_data["next_run"] > 0

    # Verify collector2 (without cache and timer)
    assert collector2_data["name"] == collector2_info["name"]
    assert collector2_data["config_path"] == collector2_info["config_path"]
    assert collector2_data["feature"] == "analytics"
    assert (
        collector2_data["service_name"]
        == f"rhc-collector-{collector2_info['id']}.service"
    )
    assert (
        collector2_data["timer_name"] == f"rhc-collector-{collector2_info['id']}.timer"
    )

    # Verify last_run and next_run are not present (no cache, no timer)
    assert "last_run" not in collector2_data or collector2_data.get("last_run") is None
    assert "next_run" not in collector2_data or collector2_data.get("next_run") is None


@pytest.mark.tier2
def test_collector_info_with_systemd_timer(multiple_test_collectors):
    """
    :id: a7b8c9d0-e1f2-3456-0123-456789abcdef
    :title: Verify Info method returns next_run from systemd timer
    :description:
        Test that the com.redhat.rhc.collector.Info method returns next_run
        timestamp when a systemd timer is enabled for the collector.
    :tags: Tier 2
    :steps:
        1. Create test collector with systemd timer and service
        2. Enable and start the timer
        3. Call com.redhat.rhc.collector.Info
        4. Verify next_run field is present and valid
    :expectedresults:
        1. Collector and timer are created
        2. Timer is enabled successfully
        3. Info call succeeds
        4. next_run field contains valid future timestamp
    """
    collector_info = multiple_test_collectors["collector1"]

    response = run_varlinkctl(
        "com.redhat.rhc.collector.Info", {"id": collector_info["id"]}
    )

    assert "info" in response
    info = response["info"]

    # Verify basic fields
    assert info["id"] == collector_info["id"]
    assert info["name"] == collector_info["name"]

    # Verify next_run is present from systemd timer
    assert "next_run" in info
    assert isinstance(info["next_run"], int)
    assert info["next_run"] > 0

    # Verify last_run from cache
    assert "last_run" in info
    assert info["last_run"] == collector_info["last_run"]


@pytest.mark.tier2
def test_collector_info_without_cache_or_timer(multiple_test_collectors):
    """
    :id: b8c9d0e1-f2a3-4567-1234-56789abcdef0
    :title: Verify Info method for collector without cache or timer
    :description:
        Test that Info method returns collector data correctly even when
        no cache file or systemd timer exists.
    :tags: Tier 2
    :steps:
        1. Create test collector without cache or timer
        2. Call com.redhat.rhc.collector.Info
        3. Verify basic fields are present
        4. Verify last_run and next_run are not present
    :expectedresults:
        1. Collector is created
        2. Info call succeeds
        3. Basic fields (id, name, config_path, etc.) are correct
        4. last_run and next_run are not present
    """
    collector_info = multiple_test_collectors["collector2"]

    response = run_varlinkctl(
        "com.redhat.rhc.collector.Info", {"id": collector_info["id"]}
    )

    assert "info" in response
    info = response["info"]

    # Verify basic fields
    assert info["id"] == collector_info["id"]
    assert info["name"] == collector_info["name"]
    assert info["config_path"] == collector_info["config_path"]
    assert info["service_name"] == f"rhc-collector-{collector_info['id']}.service"
    assert info["timer_name"] == f"rhc-collector-{collector_info['id']}.timer"
    assert info.get("feature") == "analytics"

    # Verify timing fields are not present
    assert "last_run" not in info or info.get("last_run") is None
    assert "next_run" not in info or info.get("next_run") is None


@pytest.mark.tier2
def test_collector_info_multiple_calls_consistency(multiple_test_collectors):
    """
    :id: c9d0e1f2-a3b4-5678-2345-6789abcdef01
    :title: Verify Info method returns consistent data across multiple calls
    :description:
        Test that calling Info method multiple times for the same collector
        returns consistent data.
    :tags: Tier 2
    :steps:
        1. Create test collector with cache and timer
        2. Call com.redhat.rhc.collector.Info multiple times
        3. Verify all responses are identical
    :expectedresults:
        1. Collector is created
        2. All Info calls succeed
        3. All responses contain the same data
    """
    collector_info = multiple_test_collectors["collector1"]

    # Call Info method three times
    responses = []
    for _ in range(3):
        response = run_varlinkctl(
            "com.redhat.rhc.collector.Info", {"id": collector_info["id"]}
        )
        responses.append(response["info"])

    # Verify all responses are identical
    for i in range(1, len(responses)):
        # Compare all fields except next_run (which might change)
        for key in [
            "id",
            "name",
            "config_path",
            "service_name",
            "timer_name",
            "feature",
            "last_run",
        ]:
            assert responses[0].get(key) == responses[i].get(
                key
            ), f"Field {key} differs between calls"
