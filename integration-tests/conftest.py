import pytest
import subprocess
import logging
import os

logger = logging.getLogger(__name__)


def pytest_configure():
    """Setting the name of service available on the system with rhc installed
    Name of the service in upstream package is 'yggdrasil'
    and downstream is 'rhcd'
    """
    proc = subprocess.run(
        ["systemctl", "status", "yggdrasil"],
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        universal_newlines=True,
    )
    if "Unit yggdrasil.service could not be found" in proc.stderr:
        pytest.service_name = "rhcd"
    else:
        pytest.service_name = "yggdrasil"


@pytest.fixture(scope="session", autouse=True)
def install_katello_rpm(test_config):
    if "satellite" in test_config.environment:
        # install katello rpm before register system against Satellite
        satellite_hostname = test_config.get("candlepin", "host")
        cmd = [
            "rpm",
            "-Uvh",
            "http://%s/pub/katello-ca-consumer-latest.noarch.rpm" % satellite_hostname,
        ]
        subprocess.check_call(cmd)
        logger.info("installing the katello rpm")
    yield
    if "satellite" in test_config.environment:
        cmd = "rpm -qa 'katello-ca-consumer*' | xargs rpm -e"
        subprocess.check_call(cmd, shell=True)
        logger.info("removing the katello rpm")


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
