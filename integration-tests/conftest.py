import pytest
import subprocess
import logging
import distro
import os

logger = logging.getLogger(__name__)

# default values so import-time access does not crash
pytest.rhel_version_tuple = (0, 0)


@pytest.hookimpl(trylast=True)
def pytest_configure(config):
    if distro.id() in {"rhel", "centos"}:
        pytest.rhel_version = distro.version()
        pytest.rhel_major_version = distro.major_version()

        version_parts = pytest.rhel_version.split(".")
        try:
            major = int(version_parts[0]) if version_parts[0] else 0
            minor = int(version_parts[1]) if len(version_parts) > 1 else 0
        except ValueError:
            major, minor = 0, 0

        pytest.rhel_minor_version = str(minor)
        # Store tuple for easy comparison
        pytest.rhel_version_tuple = (major, minor)
        print(f"RHEL version tuple: {pytest.rhel_version_tuple}")

    else:
        pytest.rhel_version = "unknown"
        pytest.rhel_major_version = "unknown"
        pytest.rhel_minor_version = "unknown"
        pytest.rhel_version_tuple = (0, 0)


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
                    logger.error(
                        "Failed to install katello RPM with both HTTPS and HTTP"
                    )
                    raise
    yield
    if "satellite" in test_config.environment:
        try:
            cmd = "rpm -qa 'katello-ca-consumer*' | xargs rpm -e"
            subprocess.check_call(cmd, shell=True)
            logger.info("Successfully removed katello rpm")
        except subprocess.CalledProcessError as e:
            logger.warning(f"Failed to remove katello rpm: {e}")
