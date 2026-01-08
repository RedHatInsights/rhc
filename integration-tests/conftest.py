import pytest
import subprocess
import logging
import distro
import os

logger = logging.getLogger(__name__)


@pytest.hookimpl(trylast=True)
def pytest_configure(config):
    if distro.id() == "rhel" or distro.id() == "centos":
        pytest.rhel_version = distro.version()
        pytest.rhel_major_version = distro.major_version()
    else:
        pytest.rhel_version = "unknown"
        pytest.rhel_major_version = "unknown"


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
    Fixture to manage rhcd service proxy configuration.
    Automatically cleans up proxy configuration after test completion.
    """
    service_name = "rhcd"
    override_dir = f"/etc/systemd/system/{service_name}.service.d"
    override_file = f"{override_dir}/proxy.conf"

    def _configure_proxy(proxy_url):
        """Configure rhcd service with proxy environment variables"""
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
            logger.info(f"rhcd service configured with proxy: {proxy_url}")
            return True

        except Exception as e:
            logger.error(f"Error configuring rhcd proxy: {e}")
            return False

    # Yield the configuration function
    yield _configure_proxy

    # Teardown: Clean up rhcd proxy configuration
    try:
        if os.path.exists(override_file):
            os.remove(override_file)
            subprocess.run(["systemctl", "daemon-reload"], check=True)

    except Exception as e:
        logger.error(f"Error during rhcd proxy cleanup: {e}")
