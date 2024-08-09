import pytest
import subprocess
import logging

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
        logger.info(f"installing the katello rpm")
    yield
    if "satellite" in test_config.environment:
        cmd = "rpm -qa 'katello-ca-consumer*' | xargs rpm -e"
        subprocess.check_call(cmd, shell=True)
        logger.info(f"removing the katello rpm")
