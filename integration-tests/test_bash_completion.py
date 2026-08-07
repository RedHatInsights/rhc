"""
:casecomponent: rhc
:requirement: RHSS-291300
:subsystemteam: rhel-sst-csi-client-tools
:caseautomation: Automated
:upstream: Yes
"""

import os
import shutil
import subprocess

import pytest

from utils.constants import COMPLETION_SCRIPT, MINIMAL_COLLECTOR_ID

_COMPLETION_HARNESS = """\
set -euo pipefail

binary=$1
completion_script=$2
shift 2

PROG=$binary
source "$completion_script"

completion_registration=$(complete -p "$binary")
[[ "$completion_registration" =~ -F[[:space:]]+([^[:space:]]+) ]]
completion_function=${BASH_REMATCH[1]}

COMP_WORDS=("$binary" "$@")
COMP_CWORD=$((${#COMP_WORDS[@]} - 1))
COMP_LINE="${COMP_WORDS[*]}"
COMP_POINT=${#COMP_LINE}
"$completion_function"

printf '%s\\n' "${COMPREPLY[@]}"
"""

pytestmark = pytest.mark.skip(reason="bash completion is not fully implemented")


def _get_completions(words, tmp_path):
    rhc_binary = shutil.which("rhc")
    if rhc_binary is None:
        pytest.fail("rhc binary not found on PATH")

    if not os.path.isfile(COMPLETION_SCRIPT):
        pytest.fail(f"completion script not installed at {COMPLETION_SCRIPT}")

    env = os.environ.copy()
    env["HOME"] = str(tmp_path)

    args = [
        "bash",
        "--noprofile",
        "--norc",
        "-c",
        _COMPLETION_HARNESS,
        "bash",
        rhc_binary,
        COMPLETION_SCRIPT,
    ] + list(words)

    result = subprocess.run(
        args, capture_output=True, text=True, env=env, timeout=10
    )

    if result.returncode != 0:
        pytest.fail(
            f"completion harness failed for words={words!r}: "
            f"rc={result.returncode}\nstderr: {result.stderr}"
        )
    if result.stderr:
        pytest.fail(
            f"completion harness wrote to stderr for words={words!r}:\n"
            f"{result.stderr}"
        )

    output = result.stdout.strip()
    if not output:
        return []
    return sorted(output.splitlines())


@pytest.mark.tier1
@pytest.mark.parametrize(
    "words, expected",
    [
        pytest.param(
            [""],
            ["collector", "configure", "connect", "disconnect", "help", "status"],
            id="root-commands",
        ),
        pytest.param(
            ["--"],
            ["--help", "--no-color", "--version"],
            id="root-flags",
        ),
        pytest.param(
            ["connect", "--u"],
            ["--username"],
            id="connect-flag-prefix",
        ),
        pytest.param(
            ["configure", ""],
            ["features", "help"],
            id="configure-subcommands",
        ),
        pytest.param(
            ["configure", "features", ""],
            ["disable", "enable", "help", "status"],
            id="nested-subcommands",
        ),
        pytest.param(
            ["configure", "features", "en"],
            ["enable"],
            id="nested-subcommand-prefix",
        ),
        pytest.param(
            ["configure", "features", "status", "--"],
            ["--format", "--help"],
            id="nested-command-flags",
        ),
        pytest.param(
            ["collector", ""],
            ["disable", "enable", "help", "info", "list", "timers"],
            id="collector-subcommands",
        ),
        pytest.param(
            ["collector", "enable", "--"],
            ["--help", "--now"],
            id="collector-flags",
        ),
        pytest.param(
            ["collector", "enable", "com.redhat.m"],
            [MINIMAL_COLLECTOR_ID],
            id="collector-id-prefix",
        ),
    ],
)
def test_bash_completion(words, expected, tmp_path):
    """
    :id: ab9d4175-5c79-49fe-8abc-cbaf47625834
    :title: Verify bash completion returns expected candidates
    :parametrized: yes
    :description:
        Sources the installed bash completion script for rhc, simulates
        tab-completion for the given command-line words, and verifies
        that the returned candidates match the expected set.
    :tags:
    :steps:
        1. Locate the installed rhc binary and bash completion script.
        2. Run a bash harness that sources the completion script,
           verifies the completion function is properly registered,
           sets COMP_WORDS / COMP_CWORD / COMP_LINE / COMP_POINT,
           and invokes the registered completion function.
        3. Compare the printed COMPREPLY values against the expected list.
    :expectedresults:
        1. The rhc binary and completion script are found.
        2. The harness exits successfully with no stderr output.
        3. The sorted completion candidates exactly match the expected set.
    """
    actual = _get_completions(words, tmp_path)
    assert actual == sorted(expected)
