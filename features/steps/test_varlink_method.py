from behave import given, when, then, step
import behave.runner

import json
import jsonschema
from pathlib import Path

from common import run_in_context
from constants import VARLINK_SOCKET

@given("system is registered against candlepin server")
def step_impl(context: behave.runner.Context):
    """
    Check if the system is registered against the candlepin server. If not, register it.
    :param context: behave context
    :return: None
    """
    cmd = "rhc status --format json"
    run_in_context(context, cmd, can_fail=True)
    result = json.loads(context.cmd_stdout)
    if not result["rhsm_connected"]:
        # Try to load credentials from settings file
        settings = {}
        settings_file = Path(__file__).parent / "settings.json"
        try:
            with open(settings_file, 'r') as f:
                settings = json.load(f)
        except FileNotFoundError:
            pass

        # Try to load credentials from settings file, when it was not possible to read
        # settings file or some credential is missing, then use default values.
        username = settings.get("candlepin.username", "admin")
        password = settings.get("candlepin.password", "admin")
        organization = settings.get("candlepin.organization", "donaldduck")

        cmd = (
            f"rhc connect --username {username} --password {password} --organization {organization} "
            "--enable-feature content --disable-feature analytics --disable-feature remote-management"
        )
        run_in_context(context, cmd, can_fail=False)


@given("system is not registered")
def step_impl(context: behave.runner.Context):
    """
    Ensure the system is not registered. If currently registered, disconnect.
    :param context: behave context
    :return: None
    """
    cmd = "rhc status --format json"
    run_in_context(context, cmd, can_fail=True)
    result = json.loads(context.cmd_stdout)
    if result["rhsm_connected"]:
        cmd = "rhc disconnect"
        run_in_context(context, cmd, can_fail=False)

@when("varlink method is called")
def step_impl(context: behave.runner.Context):
    """
    Call a varlink method on a specified interface.
    :param context: behave context
    :return: None
    """
    varlink_interface = None
    varlink_method = None
    varlink_args = None
    counter = 0
    for row in context.table:
        varlink_interface = row["interface"]
        varlink_method = row["method"]
        varlink_args = row["arguments"]
        counter += 1
    assert counter == 1, f"Expected exactly one row in table for 'varlink method called' ({counter} provided)"

    cmd = f"varlinkctl call --no-pager --json=short {VARLINK_SOCKET} {varlink_interface}.{varlink_method} {varlink_args}"
    run_in_context(context, cmd, can_fail=False)


@then("varlink method returns")
def step_impl(context: behave.runner.Context):
    """
    Verify the return value of the varlink method.
    :param context: behave context
    :return: None
    """
    json_object = context.text
    result = json.loads(context.cmd_stdout)
    assert result == json.loads(json_object), f"Expected {json_object}, got {result}"

@step("method returned JSON compliant with '{json_schema_doc}' schema")
def step_impl(context: behave.runner.Context, json_schema_doc):
    """

    :param context:
    :param json_schema_doc:
    :return:
    """
    with open(f"./features/json-schemas/{json_schema_doc}") as f:
        schema = json.load(f)
    result = json.loads(context.cmd_stdout)
    try:
        jsonschema.validate(instance=result, schema=schema)
    except jsonschema.ValidationError as e:
        raise AssertionError(f"JSON validation failed: {e.message}")


@then("method call was successful")
def step_impl(context: behave.runner.Context):
    """
    Verify that the varlink method call was successful.
    :param context: behave context
    :return: None
    """
    assert context.cmd_exitcode == 0, f"Method call failed with exit code {context.cmd_exitcode}"
