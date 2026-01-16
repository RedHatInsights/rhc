import pytest
import subprocess
import logging
import distro

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
