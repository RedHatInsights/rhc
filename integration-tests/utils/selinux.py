import os
import re
import subprocess
from datetime import datetime


def _get_selinux_mode():
    """Get current SELinux mode (Enforcing, Permissive, or Disabled)."""
    try:
        result = subprocess.run(["getenforce"], capture_output=True, text=True, check=True)
        return result.stdout.strip()
    except (subprocess.CalledProcessError, FileNotFoundError):
        return "Disabled"


def is_selinux_disabled():
    """Check if SELinux is disabled."""
    return _get_selinux_mode() == "Disabled"


def label_type(path):
    """Get the type of a SELinux label for a given path."""
    context = subprocess.check_output(["stat", "-c", "%C", path], text=True).strip()
    context_parts = context.split(":")
    if len(context_parts) < 4:
        raise AssertionError(f"{path} has no SELinux type in context '{context}'")
    return context_parts[2]


class AuditLogEntry:
    def __init__(self, keys, values):
        self.fields = dict(zip(keys, values))

    def __getitem__(self, item):
        return self.fields[item]

    def __str__(self):
        return subprocess.run(
            ["ausearch", "-i", "-a", f"{self.serial}"], stdout=subprocess.PIPE, check=True
        ).stdout.decode()

    @property
    def serial(self):
        return self["event"]

    @property
    def summary(self):
        return " ".join(f"{key}={value}" for key, value in self.fields.items())


class SELinuxAVCChecker:
    """Context manager for checking SELinux avc during a time period.
    This context manager automatically tracks start_time and end_time,
    removing the need for manual time tracking in tests.
    """

    def __init__(self):
        self.start_time = None
        self.end_time = None
        self.avc_skiplist = []

    def __enter__(self):
        self.start_time = datetime.now()
        return self

    def __exit__(self, exc_type, exc_val, exc_tb):
        self.end_time = datetime.now()
        return False

    def skip_avc_re(self, expression):
        expression = re.compile(expression)
        condition = lambda entry: re.search(expression, str(entry))  # noqa: E731
        self.avc_skiplist.append(condition)
        return condition

    def skip_avc_entry_by_fields(self, fields):
        condition = lambda entry: all(  # noqa: E731
            entry[key] == value for key, value in fields.items()
        )
        self.avc_skiplist.append(condition)
        return condition

    def skip_all_avcs(self):
        condition = lambda entry: True  # noqa: E731
        self.avc_skiplist.append(condition)
        return condition

    # aureport's exact separator row, used both to find the header and to
    # skip repeated separators in the data section below.
    _SEPARATOR = "=" * 63

    def get_avcs(self, skiplisted=True):
        # aureport parses --start/--end dates according to LC_TIME, not a fixed
        # format. Force the locale to match the "%m/%d/%Y" strings we generate
        # below, otherwise aureport fails with "Error parsing start date" on
        # any system where LC_TIME isn't (or doesn't resolve to) en_US-style
        # formatting. See https://bugzilla.redhat.com/show_bug.cgi?id=456441
        env = {**os.environ, "LC_TIME": "en_US.UTF-8"}
        result = subprocess.run(
            self.aureport_command, stdout=subprocess.PIPE, stderr=subprocess.PIPE, env=env
        )
        # aureport (like ausearch, which it shares exit-status semantics with)
        # exits 1 both when nothing was found and on minor argument/file
        # errors, so a non-zero exit alone isn't a reliable failure signal.
        # A clean "nothing found" run leaves stderr empty, so only treat this
        # as a hard failure when aureport actually wrote something to stderr.
        stderr = result.stderr.decode(errors="replace").strip()
        if result.returncode != 0 and stderr:
            raise RuntimeError(f"aureport failed (exit {result.returncode}): {stderr}")

        output = result.stdout.decode()
        lines = [line for line in output.splitlines() if line.strip()]

        if not lines or "<no events of interest were found>" in output:
            return

        # Find the column-header line: it sits between two separator rows.
        keys = None
        data_start = None
        for i, line in enumerate(lines):
            if line == self._SEPARATOR and i + 2 < len(lines) and lines[i + 2] == self._SEPARATOR:
                keys = lines[i + 1].split()
                data_start = i + 3
                break

        if keys is None or data_start is None:
            raise RuntimeError(f"unrecognized aureport output format:\n{output}")

        for line in lines[data_start:]:
            if line == self._SEPARATOR:
                continue
            entry = AuditLogEntry(keys, line.split())
            if skiplisted and any(condition(entry) for condition in self.avc_skiplist):
                continue
            yield entry

    @property
    def start_aureport_time(self):
        return self.start_time.strftime("%m/%d/%Y"), self.start_time.strftime("%H:%M:%S")

    @property
    def end_aureport_time(self):
        return self.end_time.strftime("%m/%d/%Y"), self.end_time.strftime("%H:%M:%S")

    @property
    def aureport_command(self):
        cmd = [
            "aureport",
            "--avc",
            "--interpret",
            "--start",
            *self.start_aureport_time,
        ]
        if self.end_time:
            cmd += ["--end", *self.end_aureport_time]
        return cmd