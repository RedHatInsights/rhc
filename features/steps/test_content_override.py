from behave import given, then
import behave.runner
import configparser

import json

from constants import DNF5_REDHAT_REPOS_OVERRIDE_FILE


@given("local DNF5 repo override file exists with content")
def step_impl(context: behave.runner.Context):
    """
    Write the given content to the DNF5 repo override file.
    Requires @fixture.no_redhat_dnf5_override_installed tag on the scenario.
    :param context: behave context
    :return: None
    """
    with open(DNF5_REDHAT_REPOS_OVERRIDE_FILE, "w") as f:
        f.write(context.text.strip() + "\n")


@given("local DNF5 repo override file is empty")
def step_impl(context: behave.runner.Context):
    """
    Create an empty DNF5 repo override file.
    Requires @fixture.no_redhat_dnf5_override_installed tag on the scenario.
    :param context: behave context
    :return: None
    """
    with open(DNF5_REDHAT_REPOS_OVERRIDE_FILE, "w") as f:
        f.write("")


@then("downloaded content overrides contain label '{label}'")
def step_impl(context: behave.runner.Context, label: str):
    """
    Verify that the Download response contains a content override with the given label.
    :param context: behave context
    :param label: content label expected among the downloaded content overrides
    :return: None
    """
    result = json.loads(context.cmd_stdout)
    assert "content_overrides" in result, (
        f"Response missing 'content_overrides' key: {result}"
    )
    labels = {o["content_label"] for o in result["content_overrides"]}
    assert label in labels, (
        f"Expected label '{label}' not found in content overrides. "
        f"Available labels: {labels}"
    )


@then("local DNF5 repo override file contains section '{section}'")
def step_impl(context: behave.runner.Context, section: str):
    """
    Verify the local DNF5 repo override file contains an INI section with the given name.
    :param context: behave context
    :param section: INI section name expected in the override file
    :return: None
    """
    config = configparser.ConfigParser()
    config.read(DNF5_REDHAT_REPOS_OVERRIDE_FILE)
    assert section in config.sections(), (
        f"Expected section '{section}' not found in override file. "
        f"Sections found: {config.sections()}"
    )


@then("local DNF5 repo override file has '{key}' set to '{value}' in section '{section}'")
def step_impl(context: behave.runner.Context, key: str, value: str, section: str):
    """
    Verify a specific key-value pair exists in the given section of the override file.
    :param context: behave context
    :param key: INI key to check
    :param value: expected value for the key
    :param section: INI section name containing the key
    :return: None
    """
    config = configparser.ConfigParser()
    config.read(DNF5_REDHAT_REPOS_OVERRIDE_FILE)
    assert section in config.sections(), (
        f"Section '{section}' not found in override file. "
        f"Sections found: {config.sections()}"
    )
    actual = config.get(section, key, fallback=None)
    assert actual == value, (
        f"Expected [{section}] {key} = {value}, got {actual}"
    )


@then("local DNF5 repo override file is empty")
def step_impl(context: behave.runner.Context):
    """
    Verify the local DNF5 repo override file is empty or contains no INI sections.
    :param context: behave context
    :return: None
    """
    config = configparser.ConfigParser()
    config.read(DNF5_REDHAT_REPOS_OVERRIDE_FILE)
    assert len(config.sections()) == 0, (
        f"Expected no sections in override file, found: {config.sections()}"
    )
