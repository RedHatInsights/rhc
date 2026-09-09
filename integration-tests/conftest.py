import pytest
import subprocess
import logging
import os
import json
import shutil
import tempfile
import time
import textwrap
from contextlib import suppress
from functools import lru_cache

from utils.systemctl import is_unit_active, is_unit_enabled
from utils.constants import (
    COLLECTOR_BIN_DIR,
    COLLECTOR_CONFIG_DIR,
    MINIMAL_COLLECTOR_CONFIG_PATH,
    MINIMAL_COLLECTOR_ID,
    MINIMAL_COLLECTOR_NAME,
    MINIMAL_SERVICE_UNIT,
    MINIMAL_TIMER_UNIT,
    RHC_SERVER_SOCKET,
    TIMER_CACHE_DIR,
    YGGDRASIL_SERVICE_NAME,
    EXIT_CODE_MOCK_MINIMAL_COLLECTOR_EXECUTABLE,
)

logger = logging.getLogger(__name__)


@lru_cache()
def is_bootc_system():
    """
    Check if the system is a bootc enabled system.
    This function duplicates the logic from pytest-client-tools' is_bootc_system fixture
    so it can be used in pytest.skipif decorators (which run at collection time).
    """
    try:
        bootc_status = subprocess.run(
            ["bootc", "status", "--format", "humanreadable"],
            capture_output=True,
            text=True,
        )
        return (bootc_status.returncode == 0) and (
            not bootc_status.stdout.strip().startswith("System is not deployed via bootc")
        )
    except FileNotFoundError:
        return False


@pytest.fixture
def writable_collector_dirs():
    """
    Ensure COLLECTOR_CONFIG_DIR and COLLECTOR_BIN_DIR are writable.

    On bootc/image-mode systems where /usr is read-only, an overlayfs is
    mounted over each directory so test fixtures can create and remove files
    there without touching the underlying read-only filesystem.  On ordinary
    systems the fixture is a no-op.

    Teardown unmounts the overlays in reverse order, automatically discarding
    any files the test created.
    """
    if not is_bootc_system():
        yield
        return

    mounted = []
    tmpdirs = []

    try:
        for target in [COLLECTOR_CONFIG_DIR, COLLECTOR_BIN_DIR]:
            tmpdir = tempfile.mkdtemp(prefix="rhc-overlay-")
            tmpdirs.append(tmpdir)
            upperdir = os.path.join(tmpdir, "upper")
            workdir = os.path.join(tmpdir, "work")
            os.makedirs(upperdir)
            os.makedirs(workdir)

            subprocess.run(
                [
                    "mount", "-t", "overlay", "overlay",
                    "-o", f"lowerdir={target},upperdir={upperdir},workdir={workdir}",
                    target,
                ],
                check=True,
                capture_output=True,
            )
            mounted.append(target)
    finally:
        yield

    for target in reversed(mounted):
        subprocess.run(["umount", target], check=False, capture_output=True)
    for tmpdir in tmpdirs:
        shutil.rmtree(tmpdir, ignore_errors=True)


@pytest.fixture(scope="module")
def rhc_server_socket():
    """
    Fixture to ensure rhc-server.socket is enabled and running before collector tests.
    This is required for varlinkctl to communicate with the rhc-server.
    """
    was_active = is_unit_active(RHC_SERVER_SOCKET)

    if not was_active:
        subprocess.run(
            ["systemctl", "enable", "--now", RHC_SERVER_SOCKET],
            check=True,
            capture_output=True,
        )

    yield

    if not was_active:
        subprocess.run(
            ["systemctl", "disable", "--now", RHC_SERVER_SOCKET],
            check=False,
            capture_output=True,
        )


@pytest.fixture
def collector_config():
    """
    Fixture providing a runnable collector using the shipped com.redhat.minimal.
    Clears any existing timer cache before the test and removes it afterwards.
    No writes to /usr — compatible with image-mode systems.
    """
    cache_path = os.path.join(TIMER_CACHE_DIR, f"{MINIMAL_COLLECTOR_ID}.json")
    if os.path.exists(cache_path):
        os.remove(cache_path)

    yield {
        "id": MINIMAL_COLLECTOR_ID,
        "name": MINIMAL_COLLECTOR_NAME,
        "config_path": MINIMAL_COLLECTOR_CONFIG_PATH,
        "bin_path": os.path.join(COLLECTOR_BIN_DIR, MINIMAL_COLLECTOR_ID),
    }

    if os.path.exists(cache_path):
        os.remove(cache_path)


@pytest.fixture
def collector_config_no_timer(writable_collector_dirs):
    """
    Fixture providing a collector that has a config file but NO systemd timer/service units.
    Used by tests that verify 'enable' fails when the timer unit is absent.

    Because the collector config must live under COLLECTOR_CONFIG_DIR (/usr/lib/rhc/collectors)
    and that path is read-only on image-mode systems, writable_collector_dirs mounts an
    overlayfs there so the file can be created and cleaned up safely.
    """
    collector_id = "test.integration.collector"
    collector_config_path = os.path.join(COLLECTOR_CONFIG_DIR, f"{collector_id}.toml")

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

    os.makedirs(COLLECTOR_CONFIG_DIR, exist_ok=True)
    with open(collector_config_path, "w") as f:
        f.write(config_content)

    yield {
        "id": collector_id,
        "name": "Test Integration Collector",
        "config_path": collector_config_path,
    }

    if os.path.exists(collector_config_path):
        os.remove(collector_config_path)


@pytest.fixture
def collector_minimal(writable_collector_dirs):
    """
    Fixture creating a minimal collector with only a config file — no binary,
    no cache, no systemd units.

    Uses writable_collector_dirs to overlay COLLECTOR_CONFIG_DIR so the config
    file can be written on bootc/image-mode systems where /usr is read-only.
    """
    collector_id = "test.collector1"
    collector_name = "Test Minimal Collector"
    config_path = os.path.join(COLLECTOR_CONFIG_DIR, f"{collector_id}.toml")

    config_content = textwrap.dedent("""
        [meta]
        name = "Test Minimal Collector"
        feature = "analytics"
        type = "ingress"

        [ingress]
        user = "root"
        group = "root"
        content_type = "application/vnd.redhat.advisor.collection"
    """).strip()

    os.makedirs(COLLECTOR_CONFIG_DIR, exist_ok=True)
    with open(config_path, "w") as f:
        f.write(config_content)

    yield {
        "id": collector_id,
        "name": collector_name,
        "config_path": config_path,
    }

    if os.path.exists(config_path):
        os.remove(config_path)


@pytest.fixture
def minimal_collector_timer_disabled():
    """
    Ensure the shipped com.redhat.minimal timer starts disabled for the test
    and is restored to enabled + daemon-reloaded afterwards.
    """
    subprocess.run(
        ["systemctl", "disable", "--now", MINIMAL_TIMER_UNIT],
        check=False,
        capture_output=True,
    )
    subprocess.run(["systemctl", "daemon-reload"], check=True)

    yield

    subprocess.run(
        ["systemctl", "enable", "--now", MINIMAL_TIMER_UNIT],
        check=False,
        capture_output=True,
    )
    subprocess.run(["systemctl", "daemon-reload"], check=True)


@pytest.fixture
def minimal_collector_slow_service():
    """
    Override the shipped com.redhat.minimal collector service to run a
    long-lived command instead of the real collection command, so tests can
    reliably catch it "in-flight" (actively running) and verify that
    ``rhc collector disable --now`` stops it rather than letting it run to
    completion.

    Restores the original service unit and the timer's prior enabled state
    afterwards.
    """
    override_dir = f"/etc/systemd/system/{MINIMAL_SERVICE_UNIT}.d"
    override_file = os.path.join(override_dir, "99-test-slow-run.conf")

    timer_was_enabled = is_unit_enabled(MINIMAL_TIMER_UNIT)

    os.makedirs(override_dir, exist_ok=True)
    with open(override_file, "w") as f:
        # The shipped unit is Type=oneshot, whose ActiveState only ever
        # reports "activating" (never "active") while the command runs, and
        # whose start job blocks until the command exits. Override to
        # Type=simple so the unit is considered "active" as soon as it
        # starts, letting tests reliably observe and stop it mid-flight.
        f.write("[Service]\nType=simple\nExecStart=\nExecStart=/bin/sleep 60\n")
    subprocess.run(["systemctl", "daemon-reload"], check=True)

    yield

    subprocess.run(
        ["systemctl", "stop", MINIMAL_SERVICE_UNIT],
        check=False,
        capture_output=True,
    )

    if os.path.exists(override_file):
        os.remove(override_file)
    with suppress(OSError):
        os.rmdir(override_dir)
    subprocess.run(["systemctl", "daemon-reload"], check=True)

    subprocess.run(
        [
            "systemctl",
            "enable" if timer_was_enabled else "disable",
            MINIMAL_TIMER_UNIT,
        ],
        check=False,
        capture_output=True,
    )


@pytest.fixture
def minimal_collector_with_timing():
    """
    Enable the shipped com.redhat.minimal timer and seed a timer cache
    so the collector has both next_run and last_run data.
    """
    cache_dir = TIMER_CACHE_DIR
    os.makedirs(cache_dir, exist_ok=True)

    cache_path = os.path.join(cache_dir, f"{MINIMAL_COLLECTOR_ID}.json")
    last_finished_timestamp = int(time.time()) - 3600
    last_started_timestamp = last_finished_timestamp - 30

    cache_content = {
        "last_started": {"timestamp": last_started_timestamp},
        "last_finished": {"timestamp": last_finished_timestamp, "exit_code": 0},
    }
    with open(cache_path, "w") as f:
        json.dump(cache_content, f)

    subprocess.run(["systemctl", "daemon-reload"], check=True)
    subprocess.run(
        ["systemctl", "enable", "--now", MINIMAL_TIMER_UNIT],
        check=True,
    )

    yield {
        "id": MINIMAL_COLLECTOR_ID,
        "name": MINIMAL_COLLECTOR_NAME,
        "config_path": MINIMAL_COLLECTOR_CONFIG_PATH,
        "cache_path": cache_path,
        "last_run": last_finished_timestamp,
    }

    subprocess.run(
        ["systemctl", "disable", "--now", MINIMAL_TIMER_UNIT],
        check=False,
    )
    subprocess.run(["systemctl", "daemon-reload"], check=True)

    if os.path.exists(cache_path):
        os.remove(cache_path)


@pytest.fixture
def minimal_collector_timer_cache():
    """
    Fixture to create a timer cache for the shipped com.redhat.minimal collector.
    """
    cache_dir = TIMER_CACHE_DIR
    os.makedirs(cache_dir, exist_ok=True)

    cache_path = os.path.join(cache_dir, f"{MINIMAL_COLLECTOR_ID}.json")
    last_run_timestamp = int(time.time()) - 3600

    cache_content = {
        "last_started": {"timestamp": last_run_timestamp - 30},
        "last_finished": {"timestamp": last_run_timestamp, "exit_code": 0},
    }
    with open(cache_path, "w") as f:
        json.dump(cache_content, f)

    yield {"path": cache_path, "last_run": last_run_timestamp}

    if os.path.exists(cache_path):
        os.remove(cache_path)


@pytest.fixture(scope="session", autouse=True)
def install_katello_rpm(test_config):
    if "satellite" in test_config.environment:
        # install katello rpm before register system against Satellite
        satellite_hostname = test_config.get("candlepin", "host")

        # Try HTTPS first, then fall back to HTTP
        for protocol in ["https", "http"]:
            rpm_url = f"{protocol}://{satellite_hostname}/pub/katello-ca-consumer-latest.noarch.rpm"
            cmd = ["rpm", "-Uvh", rpm_url]

            try:
                subprocess.check_call(cmd)
                logger.info(f"Successfully installed katello RPM from {rpm_url}")
                break
            except subprocess.CalledProcessError as e:
                logger.warning(f"Failed to install katello RPM from {rpm_url}: {e}")
                if protocol == "http":  # Last attempt failed
                    logger.error("Failed to install katello RPM with both HTTPS and HTTP")
                    raise
    yield
    if "satellite" in test_config.environment:
        try:
            cmd = "rpm -qa 'katello-ca-consumer*' | xargs rpm -e"
            subprocess.check_call(cmd, shell=True)
            logger.info("Successfully removed katello rpm")
        except subprocess.CalledProcessError as e:
            logger.warning(f"Failed to remove katello rpm: {e}")


@pytest.fixture(scope="function")
def yggdrasil_proxy_config():
    """
    Fixture to manage yggdrasil service proxy configuration.
    Automatically cleans up proxy configuration after test completion.
    """
    override_dir = f"/etc/systemd/system/{YGGDRASIL_SERVICE_NAME}.service.d"
    override_file = f"{override_dir}/proxy.conf"

    def _configure_proxy(proxy_url):
        """Configure yggdrasil service with proxy environment variables"""
        try:
            # Create systemd override with environment variables
            os.makedirs(override_dir, exist_ok=True)
            override_content = f"""[Service]
Environment=HTTPS_PROXY={proxy_url}
Environment=HTTP_PROXY={proxy_url}
"""
            with open(override_file, "w") as f:
                f.write(override_content)

            subprocess.run(["systemctl", "daemon-reload"], check=True)
            logger.info(f"{YGGDRASIL_SERVICE_NAME} service configured with proxy: {proxy_url}")
            return True

        except Exception as e:
            logger.error(f"Error configuring yggdrasil proxy: {e}")
            return False

    # Yield the configuration function
    yield _configure_proxy

    # Teardown: Clean up yggdrasil proxy configuration
    try:
        if os.path.exists(override_file):
            os.remove(override_file)
            subprocess.run(["systemctl", "daemon-reload"], check=True)

    except Exception as e:
        logger.error(f"Error during yggdrasil proxy cleanup: {e}")


@pytest.fixture
def failing_minimal_collector_executable(writable_collector_dirs):
    """Replace com.redhat.minimal executable that exits non-zero on collect."""
    bin_path = os.path.join(COLLECTOR_BIN_DIR, MINIMAL_COLLECTOR_ID)
    backup_path = bin_path + ".bak"
    script = textwrap.dedent(f"""\
        #!/bin/bash
        if [ "$1" = "collect" ]; then
            echo "forced collect failure" >&2
            exit {EXIT_CODE_MOCK_MINIMAL_COLLECTOR_EXECUTABLE}
        fi
        echo "usage: $0 collect" >&2
        exit 64
    """)

    if os.path.exists(backup_path):
        if os.path.exists(bin_path):
            os.remove(bin_path)
        os.rename(backup_path, bin_path)

    os.rename(bin_path, backup_path)
    try:
        with open(bin_path, "w") as f:
            f.write(script)
        os.chmod(bin_path, 0o755)
        yield
    finally:
        if os.path.exists(backup_path):
            if os.path.exists(bin_path):
                os.remove(bin_path)
            os.rename(backup_path, bin_path)


@pytest.fixture
def missing_minimal_collector_executable(writable_collector_dirs):
    """Remove the com.redhat.minimal executable."""
    bin_path = os.path.join(COLLECTOR_BIN_DIR, MINIMAL_COLLECTOR_ID)
    backup_path = bin_path + ".bak"

    if os.path.exists(backup_path):
        if os.path.exists(bin_path):
            os.remove(bin_path)
        os.rename(backup_path, bin_path)

    try:
        if os.path.exists(bin_path):
            os.rename(bin_path, backup_path)
        yield
    finally:
        if os.path.exists(backup_path):
            if os.path.exists(bin_path):
                os.remove(bin_path)
            os.rename(backup_path, bin_path)
