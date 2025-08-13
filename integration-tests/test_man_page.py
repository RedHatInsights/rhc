"""
:casecomponent: rhc
:requirement: RHSS-291300
:subsystemteam: rhel-sst-csi-client-tools
:caseautomation: Automated
:upstream: Yes
"""

import subprocess
import pytest


def test_man_page_synopsis():
    """
    :id: 0728e81a-07c9-4edf-9bdc-5a87cd8be9da
    :title: Verify rhc man page synopsis format
    :description:
        This test checks that the synopsis section in the `rhc` man page matches the expected format.
    :tags:
    :steps:
        1.  Display the `rhc` man page content.
    :expectedresults:
        1.  The man page content includes the exact synopsis string:
            "rhc [GLOBAL OPTIONS] command [COMMAND OPTIONS] [ARGUMENTS...]"
    """

    command_op = subprocess.check_output(["man", "rhc"]).decode("utf-8")
    assert (
        "rhc [GLOBAL OPTIONS] command [COMMAND OPTIONS] " "[ARGUMENTS...]"
    ) in command_op


def test_man_page_global_options():
    """
    :id: 5f937d4a-bf2a-480f-a31f-17629e5f9143
    :title: Verify rhc man page includes global options
    :description:
        This test checks that the `rhc` man page lists the expected global options.
    :tags:
    :steps:
        1.  Display the `rhc` man page content.
    :expectedresults:
        1.  The man page content includes the global options
            "--help, -h", "--no-color", and "--version, -v".
    """

    command_op = subprocess.check_output(["man", "rhc"]).decode("utf-8")
    assert "--help, -h" in command_op
    assert "--no-color" in command_op
    assert "--version, -v" in command_op


@pytest.mark.parametrize("command", ["connect", "disconnect", "status", "help"])
def test_man_page_commands(command):
    """
    :id: 7ca5732c-9e08-42b5-bf33-169774c914a7
    :title: Verify rhc man page lists commands
    :parametrized: yes
    :description:
        This test checks that the `rhc` man page lists common commands
        like 'connect', 'disconnect', 'status', and 'help'.
    :tags:
    :steps:
        1.  Display the `rhc` man page content.
    :expectedresults:
        1.  The man page content includes the tested command strings
            ('connect', 'disconnect', 'status', 'help').
    """

    command_op = subprocess.check_output(["man", "rhc"]).decode("utf-8")
    assert command in command_op


@pytest.mark.parametrize(
    "options",
    [
        ["--activation-key", "-a"],
        ["--organization", "-o"],
        ["--password", "-p"],
        ["--username", "-u"],
        ["--enable-feature", "-e"],
        ["--disable-feature", "-d"],
        ["--content-template", "-c"],
    ],
)
def test_man_page_connect_options(options):
    """
    :id: 762c075e-2e76-4111-b138-1d5b41e08b53
    :title: Verify rhc man page lists connect command options
    :parametrized: yes
    :description:
        This test checks that the `rhc` man page lists the expected options for the `connect` command.
    :tags:
    :steps:
        1.  Display the `rhc` man page content.
    :expectedresults:
        1.  The man page content includes all the tested options for the `connect`
            command (e.g., "--activation-key, -a", etc.).
    """

    command_op = subprocess.check_output(["man", "rhc"]).decode("utf-8")
    for option in options:
        assert option in command_op

@pytest.mark.parametrize(
    "feature_id",
    [
        "content",
        "analytics",
        "remote-management",
    ]
)
def test_man_page_feature_ids(feature_id):
    """
    :id: 9f8a4022-4a61-4439-b0ff-88a3a488f129
    :title: Verify rhc man page lists feature IDs
    :parametrized: yes
    :description:
        This test checks that the `rhc` man page lists common feature IDs
        like 'content', 'analytics', and 'remote-management'.
    :tags:
    :steps:
        1.  Display the `rhc` man page content.
    :expectedresults:
        1.  The man page content includes the tested feature ID strings
            ('content', 'analytics', 'remote-management').
    """

    command_op = subprocess.check_output(["man", "rhc"]).decode("utf-8")
    assert feature_id in command_op
