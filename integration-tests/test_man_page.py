"""
This Python module contains integration tests for rhc.
It uses pytest-client-tools Python module. More information about
this module could be found: https://github.com/ptoscano/pytest-client-tools/
"""

import subprocess
import pytest


def test_man_page_synopsis():
    """This test verifies format of man page"""
    command_op = subprocess.check_output(["man", "rhc"]).decode("utf-8")
    assert (
        "rhc [GLOBAL OPTIONS] command [COMMAND OPTIONS] " "[ARGUMENTS...]"
    ) in command_op


def test_man_page_global_options():
    """Test verifies global option are present in man page"""
    command_op = subprocess.check_output(["man", "rhc"]).decode("utf-8")
    assert "--help, -h" in command_op
    assert "--no-color" in command_op
    assert "--version, -v" in command_op


@pytest.mark.parametrize("command", ["connect", "disconnect", "status", "help"])
def test_man_page_commands(command):
    """Test verifies if man page displays existing commands"""
    command_op = subprocess.check_output(["man", "rhc"]).decode("utf-8")
    assert command in command_op


@pytest.mark.parametrize(
    "option",
    [
        "--activation-key, -a",
        "--organization, -o",
        "--password, -p",
        "--server",
        "--username, -u",
    ],
)
def test_man_page_connect_options(option):
    """Test verifies if man page displays existing options for commands"""
    command_op = subprocess.check_output(["man", "rhc"]).decode("utf-8")
    if option == "--server":
        assert option not in command_op
    else:
        assert option in command_op
