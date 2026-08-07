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

from behave import fixture

from steps.constants import OVERRIDE_DIR, OVERRIDE_FILE


@fixture
def dnf5_repo_override_file(context):
    """
    Backup and restore the DNF5 repo override file around a scenario.
    Activated via the @fixture.dnf5_override tag in feature files.
    :param context: behave context
    :return: path of the override file
    """
    os.makedirs(OVERRIDE_DIR, exist_ok=True)

    backup_file = None
    if os.path.exists(OVERRIDE_FILE):
        backup_file = OVERRIDE_FILE + ".behave-backup"
        shutil.copy2(OVERRIDE_FILE, backup_file)

    yield OVERRIDE_FILE

    if backup_file and os.path.exists(backup_file):
        shutil.move(backup_file, OVERRIDE_FILE)
    elif os.path.exists(OVERRIDE_FILE):
        os.remove(OVERRIDE_FILE)


fixture_registry = {
    "fixture.dnf5_override": dnf5_repo_override_file,
}
