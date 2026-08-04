import pytest
import subprocess
import logging
import os
import json
import time
import textwrap

from utils.systemctl import is_service_active

logger = logging.getLogger(__name__)

RHC_COLLECTOR = "/usr/libexec/rhc/rhc-collector"
TIMER_CACHE_DIR = "/var/cache/rhc/collectors"

MINIMAL_COLLECTOR_ID = "com.redhat.minimal"
MINIMAL_COLLECTOR_NAME = "Minimal Host Inventory Collector"
MINIMAL_COLLECTOR_CONFIG_PATH = "/usr/lib/rhc/collectors/com.redhat.minimal.toml"
MINIMAL_TIMER_UNIT = f"rhc-collector-{MINIMAL_COLLECTOR_ID}.timer"
MINIMAL_SERVICE_UNIT = f"rhc-collector-{MINIMAL_COLLECTOR_ID}.service"


@pytest.fixture(scope="module")
def rhc_server_socket():
    """
    Fixture to ensure rhc-server.socket is enabled and running before collector tests.
    This is required for varlinkctl to communicate with the rhc-server.
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


@pytest.fixture
def collector_config():
    """
    Fixture to create a test collector configuration and binary
    that has NO systemd timer/service units.
    Used by tests that need a collector without systemd units
    (e.g. testing the 'missing timer' error path).
    """
    collector_config_dir = "/usr/lib/rhc/collectors"
    collector_bin_dir = "/usr/libexec/rhc/collectors"
    collector_id = "test.integration.collector"
    collector_config_path = os.path.join(collector_config_dir, f"{collector_id}.toml")
    collector_bin_path = os.path.join(collector_bin_dir, collector_id)

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

    collector_script = textwrap.dedent("""
        #!/bin/bash
        if [ "$1" = "collect" ]; then
            echo "test data" > "test-output.txt"
            exit 0
        else
            echo "Usage: $0 collect"
            exit 1
        fi
    """).strip()

    os.makedirs(collector_config_dir, exist_ok=True)
    with open(collector_config_path, "w") as f:
        f.write(config_content)

    os.makedirs(collector_bin_dir, exist_ok=True)
    with open(collector_bin_path, "w") as f:
        f.write(collector_script)
    os.chmod(collector_bin_path, 0o755)

    yield {
        "id": collector_id,
        "name": "Test Integration Collector",
        "config_path": collector_config_path,
        "bin_path": collector_bin_path,
    }

    if os.path.exists(collector_config_path):
        os.remove(collector_config_path)
    if os.path.exists(collector_bin_path):
        os.remove(collector_bin_path)


@pytest.fixture
def collector_minimal():
    """
    Fixture to create a minimal collector with only a config file.
    No binary, no cache, no systemd units.
    """
    collector_dir = "/usr/lib/rhc/collectors"
    os.makedirs(collector_dir, exist_ok=True)

    collector_id = "test.collector1"
    collector_name = "Test Minimal Collector"

    config_path = os.path.join(collector_dir, f"{collector_id}.toml")
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
    service_name = "yggdrasil"
    override_dir = f"/etc/systemd/system/{service_name}.service.d"
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
            logger.info(f"Yggdrasil service configured with proxy: {proxy_url}")
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
