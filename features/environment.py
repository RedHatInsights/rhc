"""
This module contains environment setup and teardown functions for the test suite.
"""

import os
import shutil
import tempfile

ENTITLEMENT_CERT_DIR = "/etc/pki/entitlement/"
ENTITLEMENT_BACKUP_DIR_PREFIX = "entitlement-backup-"
RELEASEVER_FILE = "/etc/dnf/vars/releasever"
RHSM_HOST_CONFIG_DIR = "/etc/rhsm-host"
ENTITLEMENT_HOST_CERT_DIR = "/etc/pki/entitlement-host"


def before_scenario(context, scenario) -> None:
    """
    This function is executed before each scenario in the test suite.
    Make sure that rhsm-server.socket is running
    :param context: Context object
    :param scenario: Scenario object
    :return: None
    """
    pass


def after_scenario(context, scenario) -> None:
    """
    This function is executed after each scenario in the test suite.
    :param context: Context object
    :param scenario: Scenario object
    :return: None
    """
    pass


def before_step(context, step) -> None:
    """
    This function is executed before each step in the test suite.
    :param context: Context object
    :param step: Step object
    :return: None
    """
    pass


def after_step(context, step) -> None:
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
        # TODO: Print logs of rhc-server since the scenario was started, which
        #       could be useful for debugging
