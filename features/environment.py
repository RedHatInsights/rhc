"""
This module contains environment setup and teardown functions for the test suite.
"""

import os
import shutil
import tempfile

from behave import fixture, use_fixture
import behave.runner

from behave.fixture import use_fixture_by_tag
from fixtures import fixture_registry

ENTITLEMENT_CERT_DIR = "/etc/pki/entitlement/"
ENTITLEMENT_BACKUP_DIR_PREFIX = "entitlement-backup-"
RELEASEVER_FILE = "/etc/dnf/vars/releasever"
RHSM_HOST_CONFIG_DIR = "/etc/rhsm-host"
PRODUCT_CERT_DIR = "/etc/pki/product/"
DEFAULT_PRODUCT_CERT_DIR = "/etc/pki/product-default/"
ENTITLEMENT_HOST_CERT_DIR = "/etc/pki/entitlement-host/"
RHC_SERVER_LOG_FILE = "/var/log/rhc/rhc-server.log"
DNF5_REPOS_OVERRIDE_DIR = "/etc/dnf/repos.override.d"
DNF5_REDHAT_REPOS_OVERRIDE_FILE = os.path.join(DNF5_REPOS_OVERRIDE_DIR, "98-redhat.repo")


def before_tag(context, tag) -> None:
    """
    This function is executed before each tag in the test suite.
    It is used to activate fixtures based on tags.
    :param context: Context object
    :param tag: Tag string
    :return: None
    """
    if tag.startswith("fixture."):
        return use_fixture_by_tag(tag, context, fixture_registry)


def before_scenario(context: behave.runner.Context, scenario) -> None:
    """
    This function is executed before each scenario in the test suite.
    :param context: Context object
    :param scenario: Scenario object
    :return: None
    """

    context.log_lines_before = 0
    if os.path.exists(RHC_SERVER_LOG_FILE):
        with open(RHC_SERVER_LOG_FILE, 'r') as f:
            counter = 0
            for _ in f:
                counter += 1
            context.log_lines_before = counter


def after_scenario(context: behave.runner.Context, scenario) -> None:
    """
    This function is executed after each scenario in the test suite.
    :param context: Context object
    :param scenario: Scenario object
    :return: None
    """
    pass


def before_step(context: behave.runner.Context, step) -> None:
    """
    This function is executed before each step in the test suite.
    :param context: Context object
    :param step: Step object
    :return: None
    """
    pass


def after_step(context: behave.runner.Context, step) -> None:
    """
    This function is executed after each step in the test suite.
    It checks if the step failed, and if so, it tries to print stdout and stderr
    of the failed process

    :param context: Context object
    :param step: Step object
    :return: None
    """
    if step.status == "failed":
        print(f"Step '{step.name}' failed!")
        if hasattr(context, "cmd_stdout") and context.cmd_stdout:
            print(f"context stdout: {context.cmd_stdout}")
        if hasattr(context, "cmd_stderr") and context.cmd_stderr:
            print(f"context stderr: {context.cmd_stderr}")
        # Print logs of rhc-server since the scenario was started
        if os.path.exists(RHC_SERVER_LOG_FILE):
            with open(RHC_SERVER_LOG_FILE, 'r') as f:
                counter = 0
                print("rhc-server log lines since scenario start:")
                for line in f:
                    counter += 1
                    if counter > context.log_lines_before:
                        print(line, end='')
        else:
            print(f"rhc-server log file not found: {RHC_SERVER_LOG_FILE}")


def backup_product_certs(source_dir, backup_dir):
    """
    Try to backup directory source_dir to backup_dir.
    :param source_dir: Path of source directory
    :param backup_dir: Path of backup directory
    :return: None
    """
    for filename in os.listdir(source_dir):
        src_path = str(os.path.join(source_dir, filename))
        if os.path.isfile(src_path):
            shutil.move(src_path, backup_dir)


def restore_product_certs(backup_dir, target_dir):
    """
    Try to restore the directory backup_dir to target_dir.
    :param backup_dir: Path of backup directory
    :param target_dir: Path of target directory
    :return: None
    """
    for filename in os.listdir(backup_dir):
        src_path = str(os.path.join(backup_dir, filename))
        if os.path.isfile(src_path):
            shutil.move(src_path, target_dir)
    shutil.rmtree(backup_dir)


@fixture
def no_default_product_cert_installed(context: behave.runner.Context):
    """
    Fixture that ensures that no default product certificate is installed
    in /etc/pki/product-defaul before a scenario, and it restores original
    certificates after the scenario.
    :param context: Context object
    :return: None
    """
    # Setup of fixture
    context.default_product_cert_dir_backup = tempfile.mkdtemp()
    backup_product_certs(DEFAULT_PRODUCT_CERT_DIR, context.default_product_cert_dir_backup)

    yield context.default_product_cert_dir_backup

    # Cleanup of fixture
    restore_product_certs(context.default_product_cert_dir_backup, DEFAULT_PRODUCT_CERT_DIR)

@fixture
def no_product_cert_installed(context: behave.runner.Context):
    """
    Fixture that ensures that no default product certificate is installed
    in /etc/pki/product before a scenario, and it restores original
    certificates after the scenario.
    :param context: Context object
    :return: None
    """
    # Setup of fixture
    context.product_cert_dir_backup = tempfile.mkdtemp()
    backup_product_certs(PRODUCT_CERT_DIR, context.product_cert_dir_backup)

    yield context.product_cert_dir_backup

    # Cleanup of fixture
    restore_product_certs(context.product_cert_dir_backup, PRODUCT_CERT_DIR)


PRODUCT_CERTS_TABLE = {
    "fedora-44": "644",
    "fedora-45": "645",
}


def install_default_product_cert(context: behave.runner.Context, release_id, version_id) -> str | None:
    """
    Installs default product certificate on Fedora 44, Fedora 45.
    :param context: Context object
    :param release_id: OS release ID
    :param version_id: OS release version ID
    :return: None
    """
    distro_id = release_id + "-" + version_id
    product_id = PRODUCT_CERTS_TABLE[distro_id]
    src_path = f"./features/test-data/default_product_certs/{distro_id}_{product_id}.pem"
    dst_path = f"{DEFAULT_PRODUCT_CERT_DIR}/{product_id}.pem"
    if os.path.exists(src_path):
        return shutil.copy(src_path, dst_path)
    return None


@fixture
def default_product_cert_is_installed(context: behave.runner.Context):
    """
    Fixture that ensures that at least default product certificate is installed
    in /etc/pki/product-default. This fixture is no-op on RHEL systems, because
    RHEL systems already have a default product certificate installed. This fixture
    can install a default product certificate on Fedora 44, Fedora 45.
    :param context:
    :return: None
    """
    # Read OS release information
    os_info = {}
    with open('/etc/os-release', 'r') as f:
        for line in f:
            line = line.strip()
            if '=' in line:
                key, value = line.split('=', 1)
                os_info[key] = value.strip('"')
    release_id = os_info.get('ID', None)
    version_id = os_info.get('VERSION_ID', None)
    # When RHEL is used for testing, then it is expected that the default product certificate is already installed
    if release_id == "rhel":
        return
    # Try to install default product certificate in other cases
    if release_id and version_id:
        context.default_product_cert_path = install_default_product_cert(context, release_id, version_id)
    else:
        return

    yield context.default_product_cert_path

    # Cleanup: delete the installed default product certificate
    if context.default_product_cert_path and os.path.exists(context.default_product_cert_path):
        os.remove(context.default_product_cert_path)

def before_tag(context: behave.runner.Context, tag):
    """
    This function is executed before each tag in the test suite.

    :param context: Context object
    :param tag: Tag name
    :return: None
    """
    if tag == "fixture.no_default_product_cert_installed":
        use_fixture(no_default_product_cert_installed, context)
    if tag == "fixture.no_product_cert_installed":
        use_fixture(no_product_cert_installed, context)
    if tag == "fixture.default_product_cert_is_installed":
        use_fixture(default_product_cert_is_installed, context)
