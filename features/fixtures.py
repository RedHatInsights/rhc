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
    from environment import DNF5_REPOS_OVERRIDE_DIR, DNF5_REDHAT_REPOS_OVERRIDE_FILE

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


fixture_registry = {
    "fixture.no_redhat_dnf5_override_installed": no_redhat_dnf5_override_installed,
}
