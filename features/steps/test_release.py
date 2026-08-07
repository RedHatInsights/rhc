import os

from behave import given, step
import behave.runner

from features.environment import RELEASEVER_FILE, DEFAULT_PRODUCT_CERT_DIR, PRODUCT_CERT_DIR

@given("releasever file is empty")
def step_impl(context: behave.runner.Context):
    """
    Create empty /etc/dnf/vars/releasever file
    :param context: behave context
    :return: None
    """
    with open(RELEASEVER_FILE, "w") as release_file:
        release_file.write("")


@given("releasever file is deleted")
def step_impl(context: behave.runner.Context):
    """
    Delete releasever file /etc/dnf/vars/releasever, when it exists
    :param context: behave context
    :return: None
    """
    if os.path.exists(RELEASEVER_FILE):
        os.remove(RELEASEVER_FILE)


@step("releasever file contains")
def step_impl(context: behave.runner.Context):
    """
    Create releasever file /etc/dnf/vars/releasever with given content
    :param context: behave context
    :return: None
    """
    with open(RELEASEVER_FILE, "w") as release_file:
        release_file.write(context.text)


@step("releasever file contains expected content")
def step_impl(context: behave.runner.Context):
    """
    Verify that releasever file /etc/dnf/vars/releasever contains expected content
    :param context: behave context
    :return: None
    """
    with open(RELEASEVER_FILE, "r") as release_file:
        assert context.text in release_file.read()


@step("releasever file does not exists")
def step_impl(context: behave.runner.Context):
    """
    Verify that releasever file /etc/dnf/vars/releasever does not exist
    :param context: behave context
    :return: None
    """
    assert not os.path.exists(RELEASEVER_FILE)


@step("system has no default product certificate installed")
def step_impl(context: behave.runner.Context):
    """
    Verify that no product certificate is installed
    :param context: behave context
    :return: None
    """
    assert os.path.exists(DEFAULT_PRODUCT_CERT_DIR), f"Directory {DEFAULT_PRODUCT_CERT_DIR} does not exist"
    assert len(os.listdir(DEFAULT_PRODUCT_CERT_DIR)) == 0, f"Directory {DEFAULT_PRODUCT_CERT_DIR} is not empty"


@step("system has no product certificate installed")
def step_impl(context: behave.runner.Context):
    """
    Verify that no product certificate is installed
    :param context: behave context
    :return: None
    """
    assert os.path.exists(PRODUCT_CERT_DIR), f"Directory {PRODUCT_CERT_DIR} does not exist"
    assert len(os.listdir(PRODUCT_CERT_DIR)) == 0, f"Directory {PRODUCT_CERT_DIR} is not empty"
