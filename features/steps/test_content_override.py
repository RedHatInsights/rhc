from behave import given, then
import behave.runner

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
