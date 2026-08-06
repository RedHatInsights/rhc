"""
This module contains environment setup and teardown functions for the test suite.
"""

import os

from behave.fixture import use_fixture_by_tag
from fixtures import fixture_registry

ENTITLEMENT_CERT_DIR = "/etc/pki/entitlement/"
ENTITLEMENT_BACKUP_DIR_PREFIX = "entitlement-backup-"
RELEASEVER_FILE = "/etc/dnf/vars/releasever"
RHSM_HOST_CONFIG_DIR = "/etc/rhsm-host"
ENTITLEMENT_HOST_CERT_DIR = "/etc/pki/entitlement-host"
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


def before_scenario(context, scenario) -> None:
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
