import pytest
import subprocess
import logging
import os

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


@pytest.fixture(scope="function")
def features_config_file():
    """
    Fixture to manage the RHC features drop-in configuration file.
    Creates the config directory and file, and cleans up after test completion.

    Usage:
        def test_example(features_config_file):
            features_config_file.write('features = { "content" = true }')
            # ... run test ...
            # Cleanup happens automatically after test

    The fixture provides:
        - write(content: str): Write TOML content to the config file
        - remove(): Remove the config file if it exists
        - path: The path to the config file
        - dir: The path to the config directory
    """
    RHC_CONFIG_DIR = "/etc/rhc/config.toml.d"
    RHC_FEATURES_CONFIG_FILE = f"{RHC_CONFIG_DIR}/01-features.toml"
    class FeaturesConfigManager:
        def __init__(self):
            self.dir = RHC_CONFIG_DIR
            self.path = RHC_FEATURES_CONFIG_FILE
            self._dir_existed = os.path.exists(self.dir)
            self._file_existed = os.path.exists(self.path)
            self._original_content = None
            if self._file_existed:
                with open(self.path, "r") as f:
                    self._original_content = f.read()

        def write(self, content: str):
            """Write the features configuration file with the given TOML content"""
            os.makedirs(self.dir, exist_ok=True)
            with open(self.path, "w") as f:
                f.write(content)
            logger.info(f"Created features config file: {self.path}")

        def remove(self):
            """Remove the features configuration file if it exists"""
            if os.path.exists(self.path):
                os.remove(self.path)
                logger.info(f"Removed features config file: {self.path}")

        def cleanup(self):
            """Restore original state of config file and directory"""
            if self._file_existed and self._original_content is not None:
                # Restore original content
                with open(self.path, "w") as f:
                    f.write(self._original_content)
                logger.info(f"Restored original features config file: {self.path}")
            elif os.path.exists(self.path):
                # File was created by test, remove it
                os.remove(self.path)
                logger.info(f"Removed test-created features config file: {self.path}")

            # Only remove directory if it didn't exist before and is now empty
            if not self._dir_existed and os.path.exists(self.dir):
                try:
                    os.rmdir(self.dir)
                    logger.info(f"Removed test-created config directory: {self.dir}")
                except OSError:
                    # Directory not empty, leave it
                    pass

    manager = FeaturesConfigManager()
    yield manager

    # Teardown: Restore original state
    try:
        manager.cleanup()
    except Exception as e:
        logger.error(f"Error during features config cleanup: {e}")
