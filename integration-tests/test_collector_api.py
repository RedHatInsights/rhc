"""
:casecomponent: rhc
:requirement: RHSS-XXXXX
:subsystemteam: rhel-sst-csi-client-tools
:caseautomation: Automated
:upstream: Yes
"""

import pytest
import subprocess
import time

from utils.constants import (
    MINIMAL_COLLECTOR_CONFIG_PATH,
    MINIMAL_COLLECTOR_ID,
    MINIMAL_COLLECTOR_NAME,
    MINIMAL_SERVICE_UNIT,
    MINIMAL_TIMER_UNIT,
    VARLINK_METHOD_COLLECTOR_INFO,
    VARLINK_METHOD_COLLECTOR_LIST,
)
from utils.varlink import run_varlinkctl
from utils.systemctl import get_timer_next_trigger

pytestmark = pytest.mark.usefixtures("rhc_server_socket")


@pytest.mark.tier2
def test_collector_info_method():
    """
    :id: b2c3d4e5-f6a7-8901-bcde-f12345678901
    :title: Verify collector Info method returns details for the shipped collector
    :description:
        Test that the com.redhat.rhc.collector.Info method returns
        detailed information for the shipped com.redhat.minimal collector.
    :tags: Tier 2
    :steps:
        1. Call com.redhat.rhc.collector.Info with the shipped collector ID
        2. Verify the response structure
        3. Verify collector details match the shipped configuration
    :expectedresults:
        1. The varlink call succeeds
        2. Response contains "info" key with CollectorInfo
        3. Collector details match the shipped configuration
    """
    response = run_varlinkctl(
        VARLINK_METHOD_COLLECTOR_INFO, {"id": MINIMAL_COLLECTOR_ID}
    )

    assert "info" in response
    info = response["info"]

    assert info["id"] == MINIMAL_COLLECTOR_ID
    assert info["name"] == MINIMAL_COLLECTOR_NAME
    assert info["config_path"] == MINIMAL_COLLECTOR_CONFIG_PATH
    assert info["service_name"] == MINIMAL_SERVICE_UNIT
    assert info["timer_name"] == MINIMAL_TIMER_UNIT
    assert info.get("feature") == "analytics"


@pytest.mark.tier2
def test_collector_info_with_timer_cache(minimal_collector_timer_cache):
    """
    :id: c3d4e5f6-a7b8-9012-cdef-123456789012
    :title: Verify collector Info includes timing information from cache
    :description:
        Test that the Info method returns last_run timestamp when
        timer cache exists for the shipped collector.
    :tags: Tier 2
    :steps:
        1. Create timer cache with last run information for the shipped collector
        2. Call com.redhat.rhc.collector.Info
        3. Verify last_run field is present and correct
    :expectedresults:
        1. Timer cache is created
        2. The varlink call succeeds
        3. last_run timestamp matches cache value
    """
    expected_last_run = minimal_collector_timer_cache["last_run"]

    response = run_varlinkctl(
        VARLINK_METHOD_COLLECTOR_INFO, {"id": MINIMAL_COLLECTOR_ID}
    )

    info = response["info"]

    assert "last_run" in info
    assert info["last_run"] == expected_last_run


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
        VARLINK_METHOD_COLLECTOR_INFO, {"id": nonexistent_id}, check=False
    )

    assert result.returncode != 0
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
            VARLINK_METHOD_COLLECTOR_INFO, {"id": malformed_id}, check=False
        )

        assert result.returncode != 0, f"Expected failure for ID: {malformed_id}"
        assert (
            "InvalidParameter" in result.stderr
        ), f"Expected InvalidParameter for ID: {malformed_id}, got: {result.stderr}"


@pytest.mark.tier2
def test_collector_info_with_systemd_timer(minimal_collector_with_timing):
    """
    :id: a7b8c9d0-e1f2-3456-0123-456789abcdef
    :title: Verify Info method returns next_run from systemd timer
    :description:
        Test that the com.redhat.rhc.collector.Info method returns next_run
        timestamp when the shipped collector's systemd timer is enabled.
    :tags: Tier 2
    :steps:
        1. Enable the shipped collector's timer and seed cache
        2. Call com.redhat.rhc.collector.Info
        3. Verify next_run field is present and valid
        4. Verify last_run from cache
    :expectedresults:
        1. Timer is enabled and cache is seeded
        2. Info call succeeds
        3. next_run field contains valid future timestamp
        4. last_run matches the seeded cache value
    """
    collector_info = minimal_collector_with_timing

    response = run_varlinkctl(
        VARLINK_METHOD_COLLECTOR_INFO, {"id": collector_info["id"]}
    )

    assert "info" in response
    info = response["info"]

    assert info["id"] == collector_info["id"]
    assert info["name"] == collector_info["name"]

    assert "next_run" in info
    assert isinstance(info["next_run"], int)
    assert info["next_run"] > 0

    assert "last_run" in info
    assert info["last_run"] == collector_info["last_run"]


@pytest.mark.tier2
def test_collector_info_without_cache_or_timer(collector_minimal):
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
    collector_info = collector_minimal

    response = run_varlinkctl(
        VARLINK_METHOD_COLLECTOR_INFO, {"id": collector_info["id"]}
    )

    assert "info" in response
    info = response["info"]

    assert info["id"] == collector_info["id"]
    assert info["name"] == collector_info["name"]
    assert info["config_path"] == collector_info["config_path"]
    assert info["service_name"] == f"rhc-collector-{collector_info['id']}.service"
    assert info["timer_name"] == f"rhc-collector-{collector_info['id']}.timer"
    assert info.get("feature") == "analytics"

    assert "last_run" not in info or info.get("last_run") is None
    assert "next_run" not in info or info.get("next_run") is None


@pytest.mark.tier2
def test_collector_info_multiple_calls_consistency(minimal_collector_with_timing):
    """
    :id: c9d0e1f2-a3b4-5678-2345-6789abcdef01
    :title: Verify Info method returns consistent data across multiple calls
    :description:
        Test that calling Info method multiple times for the same collector
        returns consistent data.
    :tags: Tier 2
    :steps:
        1. Enable the shipped collector's timer and seed cache
        2. Call com.redhat.rhc.collector.Info multiple times
        3. Verify all responses are identical
    :expectedresults:
        1. Collector timer is enabled and cache is seeded
        2. All Info calls succeed
        3. All responses contain the same data
    """
    collector_info = minimal_collector_with_timing

    responses = []
    for _ in range(3):
        response = run_varlinkctl(
            VARLINK_METHOD_COLLECTOR_INFO, {"id": collector_info["id"]}
        )
        responses.append(response["info"])

    for i in range(1, len(responses)):
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


@pytest.mark.tier2
def test_collector_list_method():
    """
    :id: a1b2c3d4-e5f6-7890-abcd-ef1234567890
    :title: Verify collector List method returns all collectors
    :description:
        Test that the com.redhat.rhc.collector.List method returns
        a list of all available collectors with their details.
        The shipped com.redhat.minimal collector is always present.
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
    response = run_varlinkctl(VARLINK_METHOD_COLLECTOR_LIST)

    assert "collectors" in response
    assert isinstance(response["collectors"], list)
    assert len(response["collectors"]) > 0

    for collector in response["collectors"]:
        assert "id" in collector
        assert "name" in collector
        assert "config_path" in collector
        assert "service_name" in collector
        assert "timer_name" in collector

        assert isinstance(collector["id"], str)
        assert isinstance(collector["name"], str)
        assert isinstance(collector["config_path"], str)
        assert isinstance(collector["service_name"], str)
        assert isinstance(collector["timer_name"], str)

        if "feature" in collector:
            assert collector["feature"] is None or isinstance(
                collector["feature"], str
            )
        if "last_run" in collector:
            assert isinstance(collector["last_run"], int)
        if "next_run" in collector:
            assert isinstance(collector["next_run"], int)


@pytest.mark.tier2
def test_collector_list_includes_minimal_collector():
    """
    :id: 3b86be46-2add-4eeb-992e-6b89f9ce1cb6
    :title: Verify List method includes the shipped collector
    :description:
        Test that the shipped com.redhat.minimal collector appears
        in the List method output with correct details.
    :tags: Tier 2
    :steps:
        1. Call com.redhat.rhc.collector.List
        2. Verify the shipped collector appears in the list
        3. Verify the collector has correct details
    :expectedresults:
        1. List call succeeds
        2. Shipped collector is in the returned list
        3. Details match the shipped configuration
    """
    response = run_varlinkctl(VARLINK_METHOD_COLLECTOR_LIST)

    collectors = response["collectors"]

    shipped_collector = None
    for collector in collectors:
        if collector["id"] == MINIMAL_COLLECTOR_ID:
            shipped_collector = collector
            break

    assert (
        shipped_collector is not None
    ), f"Shipped collector {MINIMAL_COLLECTOR_ID} not found in list"

    assert shipped_collector["name"] == MINIMAL_COLLECTOR_NAME
    assert shipped_collector["config_path"] == MINIMAL_COLLECTOR_CONFIG_PATH
    assert shipped_collector.get("feature") == "analytics"


@pytest.mark.tier2
def test_collector_list_with_multiple_collectors(
    minimal_collector_with_timing, collector_minimal
):
    """
    :id: f6a7b8c9-d0e1-2345-f012-3456789abcde
    :title: Verify List method with multiple collectors and varied configurations
    :description:
        Test the com.redhat.rhc.collector.List method with the shipped
        com.redhat.minimal collector (with cache and active timer) and
        a minimal test collector (without cache or timer).
    :tags: Tier 2
    :steps:
        1. Enable timer and seed cache for the shipped collector
        2. Create a minimal test collector with config only
        3. Call com.redhat.rhc.collector.List
        4. Verify both collectors appear in the list
        5. Verify the shipped collector has last_run and next_run fields
        6. Verify the minimal collector does not have last_run or next_run
    :expectedresults:
        1. Timer is enabled and cache is seeded
        2. Minimal collector is created
        3. List call succeeds
        4. Both collectors appear in the list
        5. Shipped collector with cache shows last_run and next_run
        6. Minimal collector without cache/timer has no timing fields
    """
    collector1_info = minimal_collector_with_timing
    collector2_info = collector_minimal

    response = run_varlinkctl(VARLINK_METHOD_COLLECTOR_LIST)

    assert "collectors" in response
    collectors = response["collectors"]

    collector1_data = None
    collector2_data = None

    for collector in collectors:
        if collector["id"] == collector1_info["id"]:
            collector1_data = collector
        elif collector["id"] == collector2_info["id"]:
            collector2_data = collector

    assert collector1_data is not None, f"Collector {collector1_info['id']} not found"
    assert collector2_data is not None, f"Collector {collector2_info['id']} not found"

    assert collector1_data["name"] == collector1_info["name"]
    assert collector1_data["config_path"] == collector1_info["config_path"]
    assert collector1_data["feature"] == "analytics"
    assert collector1_data["service_name"] == MINIMAL_SERVICE_UNIT
    assert collector1_data["timer_name"] == MINIMAL_TIMER_UNIT

    assert "last_run" in collector1_data
    assert collector1_data["last_run"] == collector1_info["last_run"]

    assert "next_run" in collector1_data
    assert isinstance(collector1_data["next_run"], int)
    assert collector1_data["next_run"] > 0

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

    assert "last_run" not in collector2_data or collector2_data.get("last_run") is None
    assert "next_run" not in collector2_data or collector2_data.get("next_run") is None


@pytest.mark.tier2
def test_collector_next_run_matches_systemctl(minimal_collector_timer_disabled):
    """
    :id: f2a3b4c5-d6e7-8901-2cde-f01234567890
    :title: Verify next_run in Info matches systemctl timer schedule
    :description:
        Test that the ``next_run`` timestamp returned by the varlink
        ``Info`` method matches the next trigger time reported by
        ``systemctl list-timers`` within an acceptable tolerance.
    :tags: Tier 2
    :steps:
        1. Enable the minimal collector timer via CLI
        2. Query collector Info via varlinkctl
        3. Query ``systemctl list-timers`` for the same timer
        4. Verify both next trigger timestamps match (within tolerance)
    :expectedresults:
        1. Timer is enabled and running
        2. Info response contains ``next_run``
        3. ``systemctl list-timers`` reports a next trigger
        4. Timestamps are within 60 seconds of each other
    """
    result = subprocess.run(
        ["rhc", "collector", "enable", MINIMAL_COLLECTOR_ID],
        capture_output=True,
        text=True,
    )
    assert result.returncode == 0, f"enable failed: {result.stderr}"

    time.sleep(1)

    response = run_varlinkctl(
        VARLINK_METHOD_COLLECTOR_INFO, {"id": MINIMAL_COLLECTOR_ID}
    )
    info = response["info"]
    assert "next_run" in info, "Info response should contain next_run"
    assert isinstance(info["next_run"], int)
    varlink_next_run = info["next_run"]

    systemctl_next_run = get_timer_next_trigger(MINIMAL_TIMER_UNIT)
    assert systemctl_next_run is not None, (
        f"systemctl list-timers should report a next trigger for {MINIMAL_TIMER_UNIT}"
    )

    diff = abs(varlink_next_run - systemctl_next_run)
    assert diff <= 60, (
        f"next_run mismatch: varlink={varlink_next_run}, "
        f"systemctl={systemctl_next_run}, diff={diff}s (tolerance=60s)"
    )
