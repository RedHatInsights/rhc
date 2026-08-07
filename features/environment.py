"""
This module contains environment setup and teardown functions for the test suite.
"""

import os
import shutil
import tempfile

from behave import fixture, use_fixture
import behave.runner

ENTITLEMENT_CERT_DIR = "/etc/pki/entitlement/"
ENTITLEMENT_BACKUP_DIR_PREFIX = "entitlement-backup-"
RELEASEVER_FILE = "/etc/dnf/vars/releasever"
RHSM_HOST_CONFIG_DIR = "/etc/rhsm-host"
PRODUCT_CERT_DIR = "/etc/pki/product/"
DEFAULT_PRODUCT_CERT_DIR = "/etc/pki/product-default/"
ENTITLEMENT_HOST_CERT_DIR = "/etc/pki/entitlement-host/"
RHC_SERVER_LOG_FILE = "/var/log/rhc/rhc-server.log"


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
