"""
Behave fixtures for the rhc integration test suite.

Each fixture is a generator function decorated with @fixture.
Setup runs before yield, teardown runs after the scenario ends.
Fixtures are activated via @fixture.<name> tags in feature files
and looked up in fixture_registry by the before_tag hook in
environment.py.
"""

import os
import shutil
import tempfile

from behave import fixture
import behave.runner

from steps.constants import DNF5_REPOS_OVERRIDE_DIR, DNF5_REDHAT_REPOS_OVERRIDE_FILE, \
    DEFAULT_PRODUCT_CERT_DIR, PRODUCT_CERT_DIR


@fixture
def no_redhat_dnf5_override_installed(context):
    """
    Ensure that no Red Hat DNF5 repo override file is present during
    a scenario. If the override file already exists, it is moved to a
    backup location so the scenario starts with a clean state. On
    teardown, any override file created during the scenario is removed
    and the original backup is restored.

    Activated via the @fixture.no_redhat_dnf5_override_installed tag
    in feature files.
    :param context: behave context
    :return: path of the override file
    """

    os.makedirs(DNF5_REPOS_OVERRIDE_DIR, exist_ok=True)

    context.redhat_repo_override_backup = None
    if os.path.exists(DNF5_REDHAT_REPOS_OVERRIDE_FILE):
        context.redhat_repo_override_backup = DNF5_REDHAT_REPOS_OVERRIDE_FILE + ".behave-backup"
        shutil.move(DNF5_REDHAT_REPOS_OVERRIDE_FILE, context.redhat_repo_override_backup)

    yield DNF5_REDHAT_REPOS_OVERRIDE_FILE

    if context.redhat_repo_override_backup and os.path.exists(context.redhat_repo_override_backup):
        shutil.move(context.redhat_repo_override_backup, DNF5_REDHAT_REPOS_OVERRIDE_FILE)
    elif os.path.exists(DNF5_REDHAT_REPOS_OVERRIDE_FILE):
        os.remove(DNF5_REDHAT_REPOS_OVERRIDE_FILE)


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


PRODUCT_CERTS_TABLE = {
    "fedora-44": "644",
    "fedora-45": "645",
}


def install_default_product_cert(context: behave.runner.Context, release_id, version_id) -> str | None:
    """
    Installs default product certificate on Fedora 44, Fedora 45.
    :param context: Context object
    :param release_id: OS release ID
    :param version_id: OS release version ID
    :return: None
    """
    distro_id = release_id + "-" + version_id
    product_id = PRODUCT_CERTS_TABLE[distro_id]
    src_path = f"./features/test-data/default_product_certs/{distro_id}_{product_id}.pem"
    dst_path = f"{DEFAULT_PRODUCT_CERT_DIR}/{product_id}.pem"
    if os.path.exists(src_path):
        return shutil.copy(src_path, dst_path)
    return None


@fixture
def default_product_cert_is_installed(context: behave.runner.Context):
    """
    Fixture that ensures that at least default product certificate is installed
    in /etc/pki/product-default. This fixture is no-op on RHEL systems, because
    RHEL systems already have a default product certificate installed. This fixture
    can install a default product certificate on Fedora 44, Fedora 45.
    :param context:
    :return: None
    """
    # Read OS release information
    os_info = {}
    with open('/etc/os-release', 'r') as f:
        for line in f:
            line = line.strip()
            if '=' in line:
                key, value = line.split('=', 1)
                os_info[key] = value.strip('"')
    release_id = os_info.get('ID', None)
    version_id = os_info.get('VERSION_ID', None)
    # When RHEL is used for testing, then it is expected that the default product certificate is already installed
    if release_id == "rhel":
        return
    # Try to install default product certificate in other cases
    if release_id and version_id:
        context.default_product_cert_path = install_default_product_cert(context, release_id, version_id)
    else:
        return

    yield context.default_product_cert_path

    # Cleanup: delete the installed default product certificate
    if context.default_product_cert_path and os.path.exists(context.default_product_cert_path):
        os.remove(context.default_product_cert_path)


fixture_registry = {
    "fixture.no_redhat_dnf5_override_installed": no_redhat_dnf5_override_installed,
    "fixture.no_default_product_cert_installed": no_default_product_cert_installed,
    "fixture.no_product_cert_installed": no_product_cert_installed,
    "fixture.default_product_cert_is_installed": default_product_cert_is_installed
}
