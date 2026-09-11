import os
import configparser
import subprocess
import json

import requests
from behave import step
import behave.runner

from constants import ENTITLEMENT_CERT_DIR, RHSM_CONFIG_FILE


@step("installed entitlement certificate contains '{string}'")
def step_impl(context: behave.runner.Context, string):
    """
    Try to find at least one entitlement certificate and check if it contains '{string}'
    :param context: behave context
    :param string: string expected in the entitlement certificate
    :return: None
    """
    files = os.listdir(ENTITLEMENT_CERT_DIR)
    string_found = False
    for file_name in files:
        # Skip key files
        if file_name.endswith("-key.pem"):
            continue
        if file_name.endswith(".pem"):
            with open(os.path.join(ENTITLEMENT_CERT_DIR, file_name), "r") as f:
                content = f.read()
                if string in content:
                    string_found = True
                    break
    assert string_found, f"Entitlement certificate does not contain '{string}'"


@step("entitlement certificate is regenerated on candlepin server")
def step_impl(context: behave.runner.Context):
    """
    Try to enforce a regenerating entitlement certificate on the candlepin server. This could
    be tested only against locally running candlepin server, where we have admin access.
    :param context: behave context
    :return: None
    """
    config = configparser.ConfigParser()
    config.read(RHSM_CONFIG_FILE)

    hostname = config.get("server", "hostname")
    port = config.get("server", "port")
    prefix = config.get("server", "prefix")

    # Get consumer UUID using D-Bus
    result = subprocess.run(
        ["busctl", "call", "--json=short", "com.redhat.RHSM1", "/com/redhat/RHSM1/Consumer",
         "com.redhat.RHSM1.Consumer", "GetUuid", "s", ""],
        capture_output=True,
        text=True,
        check=True
    )

    # Parse UUID from JSON output
    json_output = json.loads(result.stdout)
    consumer_uuid = json_output["data"][0]

    # Make REST API call to regenerate certificates
    url = f"https://{hostname}:{port}{prefix}/consumers/{consumer_uuid}/certificates?lazy_regen=false"
    response = requests.put(url, auth=('admin', 'admin'), verify=False)
    response.raise_for_status()


@step("entitlement certificate is installed")
def step_impl(context: behave.runner.Context):
    """
    Check if at least one entitlement certificate is installed. If yes, then store
    entitlement certificate path in the context
    :param context: behave context
    :return: None
    """
    files = os.listdir(ENTITLEMENT_CERT_DIR)
    ent_cert_found = False
    for file_name in files:
        # Skip key files
        if file_name.endswith("-key.pem"):
            continue
        if file_name.endswith(".pem"):
            with open(os.path.join(ENTITLEMENT_CERT_DIR, file_name), "r") as f:
                context.entitlement_cert_path = os.path.join(ENTITLEMENT_CERT_DIR, file_name)
                context.entitlement_cert_inode = os.stat(context.entitlement_cert_path).st_ino
                ent_cert_found = True
                break

    if not ent_cert_found:
        raise Exception("No entitlement certificate found")


@step("entitlement certificate has different name and i-node")
def step_impl(context: behave.runner.Context):
    """
    Check if the entitlement certificate has changed name and i-node
    :param context: behave context
    :return: None
    """
    files = os.listdir(ENTITLEMENT_CERT_DIR)
    ent_cert_path = None
    ent_cert_inode = None
    for file_name in files:
        # Skip key files
        if file_name.endswith("-key.pem"):
            continue
        if file_name.endswith(".pem"):
            ent_cert_path = os.path.join(ENTITLEMENT_CERT_DIR, file_name)
            ent_cert_inode = os.stat(ent_cert_path).st_ino
            break

    if not ent_cert_path:
        raise Exception("No entitlement certificate found")

    if context.entitlement_cert_path == ent_cert_path:
        raise Exception(f"Entitlement certificate has still the same name: {context.entitlement_cert_path}")

    if context.entitlement_cert_inode == ent_cert_inode:
        raise Exception(f"Entitlement certificate has still the same i-node: {context.entitlement_cert_inode}")


@step("entitlement certificate has still the same name and i-node")
def step_impl(context: behave.runner.Context):
    """
    Check if the entitlement certificate has still the same name and i-node
    :param context: behave context
    :return: None
    """
    files = os.listdir(ENTITLEMENT_CERT_DIR)
    ent_cert_path = None
    ent_cert_inode = None
    for file_name in files:
        # Skip key files
        if file_name.endswith("-key.pem"):
            continue
        if file_name.endswith(".pem"):
            ent_cert_path = os.path.join(ENTITLEMENT_CERT_DIR, file_name)
            ent_cert_inode = os.stat(ent_cert_path).st_ino
            break

    if not ent_cert_path:
        raise Exception("No entitlement certificate found")

    if context.entitlement_cert_path != ent_cert_path:
        raise Exception(f"Entitlement certificate name has changed from {context.entitlement_cert_path} to {ent_cert_path}")

    if context.entitlement_cert_inode != ent_cert_inode:
        raise Exception(f"Entitlement certificate i-node has changed from {context.entitlement_cert_inode} to {ent_cert_inode}")
