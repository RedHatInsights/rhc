import pytest
import subprocess
import logging
import os
from pytest_client_tools.util import Version

logger = logging.getLogger(__name__)



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


@pytest.fixture
def require_rhc_logging_support(rhc_version):
    """
    Skip test when rhc logging feature is not supported.

    Use this fixture in logging tests that require rhc 0.3.9+.
    """
    if rhc_version < Version("0.3.9"):
        pytest.skip("rhc logging is supported only on rhc >= 0.3.9")


@pytest.fixture
def rhc_version(rhc):
    """
    Current rhc version as pytest_client_tools.util.Version.
    """
    return rhc.version


@pytest.fixture
def is_rhc_version_at_least(rhc_version):
    """
    Helper for inline rhc version checks in tests.
    """

    def _is_at_least(min_version: str) -> bool:
        return rhc_version >= Version(min_version)

    return _is_at_least


@pytest.fixture
def require_rhc_version_at_least(rhc_version):
    """
    Skip test when installed rhc is lower than the minimum required version.
    """

    def _require(min_version: str, reason: str = "feature not supported by installed rhc"):
        min_ver = Version(min_version)
        if rhc_version < min_ver:
            pytest.skip(f"{reason} (rhc={rhc_version}, requires>={min_ver})")

    return _require


@pytest.fixture
def require_rhsm_masked_support(require_rhc_version_at_least):
    """
    Skip RHSM-masked status tests when feature behavior is not supported.
    """
    require_rhc_version_at_least(
        "0.3.3",
        reason="RHSM masked status tests are supported only on rhc >= 0.3.3",
    )
