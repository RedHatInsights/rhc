"""Run rhc attached to a PTY so interactive UI (spinners) is enabled."""

import logging
import os
import subprocess

import pexpect
import pytest

logger = logging.getLogger(__name__)

_SECRET_FLAGS = (
    "--activation-key",
    "--organization",
    "--password",
    "--username",
)


def run_rhc_with_tty(*args, timeout=180):
    """Run ``rhc`` with stdin, stdout, and stderr attached to a PTY.

    ``rhc`` enables spinners only when stdout is a terminal. ``rhc.run()``
    uses pipes, so tests that need the TTY UI must go through this helper.

    Returns a ``subprocess.CompletedProcess`` with combined PTY output in
    ``stdout`` (text). ``stderr`` is empty because both streams share the PTY.
    """
    argv = ["rhc"] + list(args)
    env = os.environ.copy()
    env.setdefault("TERM", "xterm")
    # Keep a human-readable session: animations follow the PTY, not these vars.
    env.pop("NO_COLOR", None)

    logger.info("Running on PTY: %s", " ".join(_redact_argv(argv)))

    try:
        child = pexpect.spawn(
            argv[0],
            argv[1:],
            dimensions=(24, 80),
            timeout=timeout,
            env=env,
            encoding="utf-8",
            codec_errors="replace",
        )
    except pexpect.ExceptionPexpect as exc:
        pytest.skip(f"unable to allocate a PTY: {exc}")

    try:
        child.expect(pexpect.EOF)
    except pexpect.TIMEOUT:
        child.terminate(force=True)
        raise TimeoutError(
            f"{argv[0]} did not exit within {timeout}s; "
            f"output was:\n{child.before!r}"
        )

    output = child.before
    child.close()
    returncode = child.exitstatus
    if returncode is None:
        # Killed by a signal: mirror subprocess's negative-returncode convention.
        returncode = -(child.signalstatus or 0)

    return subprocess.CompletedProcess(
        args=argv, returncode=returncode, stdout=output, stderr=""
    )


def _redact_argv(argv):
    redacted = []
    hide_next = False
    for part in argv:
        if hide_next:
            redacted.append("<redacted>")
            hide_next = False
            continue
        redacted.append(part)
        if part in _SECRET_FLAGS:
            hide_next = True
    return redacted
