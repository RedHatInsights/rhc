"""
:casecomponent: rhc
:requirement: RHSS-291811
:subsystemteam: rhel-sst-csi-client-tools
:caseautomation: Automated
:upstream: Yes
"""

import contextlib
import os
import re
import stat
import subprocess
import pytest

from utils import prepare_args_for_connect

LOG_FILE_PATH = "/var/log/rhc/rhc.log"
LOGROTATE_CONFIG_PATH = "/etc/logrotate.d/rhc"


class LogMonitor:
    """Captures log file position before a test and provides helpers to read new entries."""

    def __init__(self):
        self.start_pos = 0
        if os.path.exists(LOG_FILE_PATH):
            self.start_pos = os.path.getsize(LOG_FILE_PATH)

    def get_new_content(self):
        if not os.path.exists(LOG_FILE_PATH):
            return ""
        with open(LOG_FILE_PATH, "r") as f:
            f.seek(self.start_pos)
            return f.read()

    def get_new_lines(self):
        return [line for line in self.get_new_content().splitlines() if line.strip()]


@pytest.fixture
def log_monitor():
    """Fixture that records the log file position before the test starts."""
    return LogMonitor()


@pytest.mark.tier1
def test_log_file_created(rhc, log_monitor):
    """
    :id: 67717c57-5458-4fa0-995d-5c921f3280fb
    :title: Verify log file is created at '/var/log/rhc/rhc.log' after 'rhc status'
    :description:
        Verifies that after running 'rhc status', the log file exists at the
        expected path /var/log/rhc/rhc.log and contains new log entries.
    :tags: Tier 1
    :steps:
        1. Run 'rhc status'.
        2. Verify that '/var/log/rhc/rhc.log' exists.
        3. Verify that the log file received new entries.
    :expectedresults:
        1. 'rhc status' runs (exit code may be non-zero if disconnected).
        2. The log file exists at '/var/log/rhc/rhc.log'.
        3. New log entries are present in the file.
    """
    rhc.run("status", check=False)

    assert os.path.exists(LOG_FILE_PATH), (
        f"Log file {LOG_FILE_PATH} should exist after rhc status"
    )
    new_content = log_monitor.get_new_content()
    assert len(new_content.strip()) > 0, (
        "Log file should contain new entries after rhc status"
    )


@pytest.mark.tier1
def test_log_file_permissions(rhc):
    """
    :id: cf08e9a7-e631-4477-a2c6-63417147919f
    :title: Verify log file is created with 640 permissions
    :description:
        Verifies that the log file '/var/log/rhc/rhc.log' has restrictive
        file permissions (640 — owner read/write, group read, no other access).
    :tags: Tier 1
    :steps:
        1. Run 'rhc status' to ensure the log file is present.
        2. Check file permissions of '/var/log/rhc/rhc.log'.
    :expectedresults:
        1. 'rhc status' runs (exit code may be non-zero if disconnected).
        2. The log file has 0640 permissions (rw-r-----).
    """
    rhc.run("status", check=False)

    assert os.path.exists(LOG_FILE_PATH), (
        f"Log file {LOG_FILE_PATH} should exist"
    )
    file_stat = os.stat(LOG_FILE_PATH)
    file_mode = stat.S_IMODE(file_stat.st_mode)
    assert file_mode == 0o640, (
        f"Log file permissions should be 0640, got {oct(file_mode)}"
    )


@pytest.fixture
def non_root_user():
    """Creates a temporary non-root user for testing, then removes it after."""
    username = "rhc_test_user"
    subprocess.run(["userdel", "-r", username], check=False)
    subprocess.run(["useradd", "-m", username], check=True)
    yield username
    subprocess.run(["userdel", "-r", username], check=False)


@pytest.mark.tier1
def test_log_file_created_non_root(non_root_user):
    """
    :id: 89da0e9d-3c14-42fd-b45d-f695de60119b
    :title: Verify non-root user log file is created at ~/.local/state/rhc/rhc.log
    :description:
        Verifies that when rhc is run as a non-root user, the log file is
        created under ~/.local/state/rhc/rhc.log (instead of /var/log/rhc/rhc.log),
        the log directory has 0700 permissions, and the log file contains
        expected startup entries.
    :tags: Tier 1
    :steps:
        1. Create a temporary non-root user.
        2. Run 'rhc status' as the non-root user.
        3. Verify that ~/.local/state/rhc/rhc.log exists for the user.
        4. Verify the log directory has 0700 permissions.
        5. Verify the log file contains startup entries.
        6. Remove the temporary user.
    :expectedresults:
        1. The temporary user is created.
        2. 'rhc status' runs (may fail for non-root, but logging still occurs).
        3. The log file exists at ~/.local/state/rhc/rhc.log.
        4. The log directory ~/.local/state/rhc has 0700 permissions.
        5. The log file contains 'rhc started' with version and pid.
        6. The temporary user is removed.
    """
    home_dir = os.path.expanduser(f"~{non_root_user}")
    log_dir = os.path.join(home_dir, ".local", "state", "rhc")
    log_path = os.path.join(log_dir, "rhc.log")

    subprocess.run(
        ["su", "-", non_root_user, "-c", "rhc status"],
        capture_output=True, text=True,
    )

    assert os.path.exists(log_path), (
        f"Non-root log file should exist at {log_path}"
    )

    dir_mode = stat.S_IMODE(os.stat(log_dir).st_mode)
    assert dir_mode == 0o700, (
        f"Non-root log directory permissions should be 0700, got {oct(dir_mode)}"
    )

    with open(log_path, "r") as f:
        content = f.read()
    assert 'msg="rhc started"' in content, (
        "Non-root log file should contain 'rhc started' message"
    )
    assert "version=" in content, (
        "Non-root log file should contain version information"
    )
    assert "pid=" in content, (
        "Non-root log file should contain PID information"
    )


@pytest.mark.tier1
def test_log_file_open_failure_message(non_root_user):
    """
    :id: 6ded3c1d-8557-476b-b156-a1425ce0705a
    :title: Verify CLI warns user when log file cannot be opened
    :description:
        Verifies that when a non-root user's home directory is read-only
        and the log directory (~/.local/state/rhc) cannot be created,
        the CLI output displays a warning message informing the user that
        detailed logs will not be available.
    :tags: Tier 1
    :steps:
        1. Create a temporary non-root user.
        2. Make the user's home directory read-only so the log directory
           cannot be created.
        3. Run 'rhc status' as the non-root user.
        4. Verify CLI output contains the warning about the unopenable log file.
        5. Restore home directory permissions and remove the user.
    :expectedresults:
        1. The temporary user is created.
        2. The home directory is read-only.
        3. 'rhc status' runs.
        4. CLI output contains "Unable to open log file:" and
           "Detailed logs will not be available."
        5. The home directory permissions are restored and the user is removed.
    """
    home_dir = os.path.expanduser(f"~{non_root_user}")
    os.chmod(home_dir, 0o555)
    try:
        result = subprocess.run(
            ["su", "-", non_root_user, "-c", "rhc status"],
            capture_output=True, text=True,
        )
        output = result.stdout + result.stderr
        assert "Unable to open log file:" in output, (
            f"CLI should warn about unopenable log file, got: {output!r}"
        )
        assert "Detailed logs will not be available." in output, (
            f"CLI should inform that logs are unavailable, got: {output!r}"
        )
    finally:
        os.chmod(home_dir, 0o755)


@pytest.mark.tier1
def test_log_contains_startup_info(rhc, log_monitor):
    """
    :id: a473f656-2bfc-4ecc-9f20-b344d8a0595f
    :title: Verify log file contains startup information (version and PID)
    :description:
        Verifies that the log file contains the expected startup log entry
        including the rhc version and process ID when any rhc command is run.
    :tags: Tier 1
    :steps:
        1. Run 'rhc status'.
        2. Read new log entries.
        3. Verify log contains 'rhc started' with version and pid attributes.
    :expectedresults:
        1. 'rhc status' runs.
        2. New log entries are captured.
        3. Log contains an entry with msg="rhc started", version=, and pid=.
    """
    rhc.run("status", check=False)

    content = log_monitor.get_new_content()
    assert 'msg="rhc started"' in content, (
        "Log should contain 'rhc started' message"
    )
    assert "version=" in content, "Log should contain version information"
    assert "pid=" in content, "Log should contain PID information"


@pytest.mark.tier1
def test_log_entries_have_timestamps(rhc, log_monitor):
    """
    :id: 21171390-e8b6-4b68-bf8c-7a50c3ce94e6
    :title: Verify all log entries include proper timestamps
    :description:
        Verifies that log entries in /var/log/rhc/rhc.log include proper
        timestamps using the slog TextHandler format (time= prefix).
    :tags: Tier 1
    :steps:
        1. Run 'rhc status'.
        2. Read new log lines.
        3. Verify that each non-empty log line starts with 'time='.
    :expectedresults:
        1. 'rhc status' runs.
        2. New log lines are captured.
        3. All non-empty log lines begin with 'time=' followed by a timestamp.
    """
    rhc.run("status", check=False)

    lines = log_monitor.get_new_lines()
    assert len(lines) > 0, "Log file should contain at least one new log entry"
    for line in lines:
        assert line.startswith("time="), (
            f"Log line should start with 'time=', got: {line!r}"
        )


@pytest.mark.tier1
def test_log_connect_command_logged(
    external_candlepin, rhc, test_config, log_monitor
):
    """
    :id: 809cee40-4a74-4456-b208-ebce3984f4ca
    :title: Verify log file records the connect command execution
    :description:
        Verifies that after running 'rhc connect', the log file contains
        entries related to the connect command including command start
        and RHSM registration messages.
    :tags: Tier 1
    :steps:
        1. Ensure the system is disconnected.
        2. Run 'rhc connect'.
        3. Read new log entries.
        4. Verify log contains connect-related messages.
    :expectedresults:
        1. The system is disconnected.
        2. 'rhc connect' executes successfully.
        3. New log entries are captured.
        4. Log contains "Command 'rhc connect' started" and
           "Registering the system with Red Hat Subscription Management".
    """
    with contextlib.suppress(Exception):
        rhc.disconnect()

    command_args = prepare_args_for_connect(test_config, auth="activation-key")
    command = ["connect"] + command_args
    rhc.run(*command)

    content = log_monitor.get_new_content()
    assert "Command 'rhc connect' started" in content, (
        "Log should contain connect command start message"
    )
    assert "Registering the system with Red Hat Subscription Management" in content, (
        "Log should contain RHSM registration message"
    )


@pytest.mark.tier1
def test_log_disconnect_command_logged(
    external_candlepin, rhc, test_config, log_monitor
):
    """
    :id: 174e3c20-7665-4a7b-9ab3-548be0dcc73d
    :title: Verify log file records the disconnect command execution
    :description:
        Verifies that after running 'rhc disconnect', the log file contains
        entries related to the disconnect command including service
        deactivation and RHSM unregistration messages.
    :tags: Tier 1
    :steps:
        1. Connect the system using 'rhc connect'.
        2. Record current log position.
        3. Run 'rhc disconnect'.
        4. Read new log entries.
        5. Verify log contains disconnect-related messages.
    :expectedresults:
        1. The system is connected.
        2. Log position is recorded.
        3. 'rhc disconnect' executes successfully.
        4. New log entries are captured.
        5. Log contains "Command 'rhc disconnect' started",
           "Deactivating the yggdrasil service",
           "Disconnecting from Red Hat Lightspeed",
           and "Unregistering system from Red Hat Subscription Management".
    """
    command_args = prepare_args_for_connect(test_config, auth="activation-key")
    command = ["connect"] + command_args
    rhc.run(*command)

    monitor = LogMonitor()

    rhc.run("disconnect")

    content = monitor.get_new_content()
    assert "Command 'rhc disconnect' started" in content, (
        "Log should contain disconnect command start message"
    )
    assert "Deactivating the yggdrasil service" in content, (
        "Log should contain yggdrasil deactivation message"
    )
    assert "Disconnecting from Red Hat Lightspeed" in content, (
        "Log should contain Lightspeed disconnection message"
    )
    assert "Unregistering system from Red Hat Subscription Management" in content, (
        "Log should contain RHSM unregistration message"
    )


@pytest.mark.tier1
def test_log_status_command_logged(rhc, log_monitor):
    """
    :id: b3993b95-d0f5-46f6-90e7-6990d0c9ed45
    :title: Verify log file records the status command execution
    :description:
        Verifies that after running 'rhc status', the log file contains
        entries for each status check performed (RHSM, content, Lightspeed,
        yggdrasil).
    :tags: Tier 1
    :steps:
        1. Run 'rhc status'.
        2. Read new log entries.
        3. Verify log contains status-related messages.
    :expectedresults:
        1. 'rhc status' runs.
        2. New log entries are captured.
        3. Log contains "Command 'status' started",
           "Checking system connection status",
           "Checking status of Red Hat Subscription Management",
           "Checking content status",
           "Checking status of Red Hat Lightspeed",
           and "Checking status of yggdrasil service".
    """
    rhc.run("status", check=False)

    content = log_monitor.get_new_content()
    assert "Command 'rhc status' started" in content, (
        "Log should contain status command start message"
    )
    assert "Checking system connection status" in content, (
        "Log should contain system status check message"
    )
    assert "Checking status of Red Hat Subscription Management" in content, (
        "Log should contain RHSM status check message"
    )
    assert "Checking content status" in content, (
        "Log should contain content status check message"
    )
    assert "Checking status of Red Hat Lightspeed" in content, (
        "Log should contain Lightspeed status check message"
    )
    assert "Checking status of yggdrasil service" in content, (
        "Log should contain yggdrasil status check message"
    )


@pytest.mark.tier1
def test_log_error_message_references_log_file(
    external_candlepin, rhc, test_config
):
    """
    :id: 5817739b-bf6b-4dfb-b704-57da7d7ab268
    :title: Verify CLI output references log file path upon connect error
    :description:
        Verifies that when 'rhc connect' encounters an error (e.g. invalid
        credentials), the CLI output includes a message directing the user
        to the log file for full details.
    :tags: Tier 1
    :steps:
        1. Ensure the system is disconnected.
        2. Run 'rhc connect' with invalid credentials.
        3. Verify the command fails with non-zero return code.
        4. Verify CLI output contains "Please see /var/log/rhc/rhc.log for full details."
    :expectedresults:
        1. The system is disconnected.
        2. 'rhc connect' fails.
        3. Return code is non-zero.
        4. CLI output includes a reference to /var/log/rhc/rhc.log.
    """
    with contextlib.suppress(Exception):
        rhc.disconnect()

    credentials = {
        "username": "non-existent-user",
        "password": "candlepin.password",
    }
    command_args = prepare_args_for_connect(test_config, credentials=credentials)
    command = ["connect"] + command_args
    result = rhc.run(*command, check=False)

    assert result.returncode != 0
    output = result.stdout + result.stderr
    assert "Please see /var/log/rhc/rhc.log for full details." in output, (
        f"CLI output should reference log file on error, got: {output!r}"
    )


@pytest.mark.tier1
def test_log_separate_runs_with_blank_line(rhc, log_monitor):
    """
    :id: 817cc8cd-7743-44bd-ab06-d2215901f9c3
    :title: Verify log entries from different runs are separated by blank lines
    :description:
        Verifies that consecutive rhc invocations produce log entries that
        are separated by blank lines, making it easier to distinguish between
        different command runs in the log file.
    :tags: Tier 1
    :steps:
        1. Run 'rhc status' twice.
        2. Read new log entries.
        3. Verify that the log contains at least two 'rhc started' entries.
        4. Verify that there is a blank line separating the two runs.
    :expectedresults:
        1. Both 'rhc status' commands run.
        2. New log entries are captured.
        3. The log contains at least two 'rhc started' entries.
        4. A blank line exists between the two runs.
    """
    rhc.run("status", check=False)
    rhc.run("status", check=False)

    content = log_monitor.get_new_content()
    occurrences = content.count('msg="rhc started"')
    assert occurrences >= 2, (
        f"Expected at least 2 'rhc started' entries, found {occurrences}"
    )
    assert "\n\n" in content, (
        "Log should contain blank lines separating different runs"
    )


@pytest.mark.tier1
def test_log_no_slog_output_in_cli(rhc):
    """
    :id: 9b54716a-dfa5-4cb3-be34-bbf600c4f61b
    :title: Verify slog messages do not appear in CLI stdout/stderr
    :description:
        Verifies that structured log messages (slog format with time=, level=,
        msg=) do not leak into the CLI stdout or stderr output, ensuring clean
        separation between file logging and user-facing output.
    :tags: Tier 1
    :steps:
        1. Run 'rhc status'.
        2. Check stdout and stderr of the command.
        3. Verify no slog-formatted messages appear in the output.
    :expectedresults:
        1. 'rhc status' runs.
        2. stdout and stderr are captured.
        3. Neither stdout nor stderr contains slog-formatted log lines.
    """
    result = rhc.run("status", check=False)

    output = result.stdout + result.stderr
    slog_pattern = re.compile(r"time=\S+ level=\w+ msg=")
    assert not slog_pattern.search(output), (
        f"CLI output should not contain slog messages, found in: {output!r}"
    )


@pytest.mark.tier1
def test_logrotate_config_exists():
    """
    :id: 590c5485-21c5-4cad-882e-de006b0dfd29
    :title: Verify logrotate configuration file exists for rhc
    :description:
        Verifies that the logrotate configuration file for rhc log files
        exists at '/etc/logrotate.d/rhc' and contains expected directives
        for daily rotation, compression, and correct file permissions.
    :tags: Tier 1
    :steps:
        1. Check if '/etc/logrotate.d/rhc' exists.
        2. Read the logrotate configuration file.
        3. Verify it contains expected directives.
    :expectedresults:
        1. The logrotate configuration file exists.
        2. The file is readable.
        3. The file contains '/var/log/rhc/*.log', 'daily', 'compress',
           'copytruncate', 'create 640 root root', 'missingok', and
           'notifempty' directives.
    """
    assert os.path.exists(LOGROTATE_CONFIG_PATH), (
        f"Logrotate config {LOGROTATE_CONFIG_PATH} should exist"
    )

    with open(LOGROTATE_CONFIG_PATH, "r") as f:
        config = f.read()

    assert "/var/log/rhc/*.log" in config, (
        "Logrotate config should target /var/log/rhc/*.log"
    )
    for directive in ("daily", "compress", "copytruncate", "missingok", "notifempty"):
        assert directive in config, (
            f"Logrotate config should include '{directive}' directive"
        )
    assert "create 640 root root" in config, (
        "Logrotate config should include 'create 640 root root' directive"
    )


@pytest.mark.tier1
def test_log_connect_error_logged(
    external_candlepin, rhc, test_config, log_monitor
):
    """
    :id: 141aba4f-9898-4881-b0fd-2b978e6af9d6
    :title: Verify log file records errors when connect fails with invalid credentials
    :description:
        Verifies that when 'rhc connect' fails due to invalid credentials,
        the error is logged at ERROR level in the log file, providing
        details useful for troubleshooting.
    :tags: Tier 1
    :steps:
        1. Ensure the system is disconnected.
        2. Run 'rhc connect' with invalid credentials.
        3. Read new log entries.
        4. Verify log contains an ERROR level entry about the connection failure.
    :expectedresults:
        1. The system is disconnected.
        2. 'rhc connect' fails.
        3. New log entries are captured.
        4. Log contains 'level=ERROR' and a message about RHSM connection failure.
    """
    with contextlib.suppress(Exception):
        rhc.disconnect()

    credentials = {
        "username": "non-existent-user",
        "password": "candlepin.password",
    }
    command_args = prepare_args_for_connect(test_config, credentials=credentials)
    command = ["connect"] + command_args
    rhc.run(*command, check=False)

    content = log_monitor.get_new_content()
    assert "level=ERROR" in content, (
        "Log should contain at least one ERROR level entry when connect fails"
    )
    assert "cannot connect to Red Hat Subscription Management" in content, (
        "Log should contain RHSM connect error details"
    )


@pytest.mark.tier1
def test_log_status_connected(
    external_candlepin, rhc, test_config, log_monitor
):
    """
    :id: dd566bab-8519-47ed-b2e5-2364585441cb
    :title: Verify log records correct status messages when system is connected
    :description:
        Verifies that when the system is connected and 'rhc status' is run,
        the log file reflects the connected state with appropriate INFO messages
        for RHSM, Lightspeed, and yggdrasil.
    :tags: Tier 1
    :steps:
        1. Connect the system using 'rhc connect'.
        2. Record current log position.
        3. Run 'rhc status'.
        4. Read new log entries.
        5. Verify log contains connected status messages.
    :expectedresults:
        1. The system is connected.
        2. Log position is recorded.
        3. 'rhc status' returns exit code 0.
        4. New log entries are captured.
        5. Log contains "Connected to Red Hat Subscription Management",
           "Connected to Red Hat Lightspeed",
           and "The yggdrasil service is active".
    """
    with contextlib.suppress(Exception):
        rhc.disconnect()

    rhc.connect(
        activationkey=test_config.get("candlepin.activation_keys")[0],
        org=test_config.get("candlepin.org"),
    )
    monitor = LogMonitor()

    status_result = rhc.run("status", check=False)
    assert status_result.returncode == 0

    content = monitor.get_new_content()
    assert "Connected to Red Hat Subscription Management" in content, (
        "Log should contain RHSM connected message"
    )
    assert "Connected to Red Hat Lightspeed" in content, (
        "Log should contain Lightspeed connected message"
    )
    assert "The yggdrasil service is active" in content, (
        "Log should contain yggdrasil active message"
    )


@pytest.mark.tier1
def test_log_status_disconnected(rhc, log_monitor):
    """
    :id: 7ec3adae-79ee-4038-b565-06f01b2cd017
    :title: Verify log records correct status messages when system is disconnected
    :description:
        Verifies that when the system is disconnected and 'rhc status' is run,
        the log file reflects the disconnected state with appropriate messages
        for RHSM and Lightspeed.
    :tags: Tier 1
    :steps:
        1. Ensure the system is disconnected.
        2. Record current log position.
        3. Run 'rhc status'.
        4. Read new log entries.
        5. Verify log contains disconnected status messages.
    :expectedresults:
        1. The system is disconnected.
        2. Log position is recorded.
        3. 'rhc status' runs with non-zero exit code.
        4. New log entries are captured.
        5. Log contains "Not connected to Red Hat Subscription Management"
           and "Not connected to Red Hat Lightspeed".
    """
    rhc.run("disconnect", check=False)
    monitor = LogMonitor()

    status_result = rhc.run("status", check=False)
    assert status_result.returncode != 0

    content = monitor.get_new_content()
    assert "Not connected to Red Hat Subscription Management" in content, (
        "Log should contain RHSM disconnected message"
    )
    assert "Not connected to Red Hat Lightspeed" in content, (
        "Log should contain Lightspeed disconnected message"
    )


@pytest.mark.tier1
def test_log_connect_with_disabled_feature(
    external_candlepin, rhc, test_config, log_monitor
):
    """
    :id: d6d5af53-8549-4ef8-a27d-79c41cdf280b
    :title: Verify log records disabled features during connect
    :description:
        Verifies that when 'rhc connect' is run with --disable-feature content,
        the log file contains messages indicating the content feature was
        disabled and the repository file was not generated.
    :tags: Tier 1
    :steps:
        1. Ensure the system is disconnected.
        2. Run 'rhc connect' with --disable-feature content.
        3. Read new log entries.
        4. Verify log contains messages about the disabled content feature.
    :expectedresults:
        1. The system is disconnected.
        2. 'rhc connect' executes.
        3. New log entries are captured.
        4. Log contains a message indicating that the content feature is
           disabled and that redhat.repo was not generated.
    """
    with contextlib.suppress(Exception):
        rhc.disconnect()

    command_args = prepare_args_for_connect(test_config, auth="activation-key")
    command_args.extend(["--disable-feature", "content"])
    command = ["connect"] + command_args
    rhc.run(*command)

    content = log_monitor.get_new_content()
    assert "redhat.repo not generated" in content or "content feature disabled" in content, (
        "Log should indicate the content feature was disabled"
    )


@pytest.mark.tier1
def test_log_connect_records_all_features(
    external_candlepin, rhc, test_config, log_monitor
):
    """
    :id: 7d7addd4-579b-462b-9759-744b2830ab27
    :title: Verify log records all connect steps: RHSM, Lightspeed, yggdrasil
    :description:
        Verifies that a full 'rhc connect' logs messages for all three main steps:
        RHSM registration, Lightspeed (Insights) connection, and yggdrasil
        service activation.
    :tags: Tier 1
    :steps:
        1. Ensure the system is disconnected.
        2. Run 'rhc connect' with activation-key.
        3. Read new log entries.
        4. Verify log contains messages for all three connect steps.
    :expectedresults:
        1. The system is disconnected.
        2. 'rhc connect' executes successfully.
        3. New log entries are captured.
        4. Log contains "Registering the system with Red Hat Subscription Management",
           a Lightspeed connection message, and an yggdrasil activation message.
    """
    with contextlib.suppress(Exception):
        rhc.disconnect()

    command_args = prepare_args_for_connect(test_config, auth="activation-key")
    command = ["connect"] + command_args
    rhc.run(*command)

    content = log_monitor.get_new_content()
    assert "Registering the system with Red Hat Subscription Management" in content, (
        "Log should contain RHSM registration step"
    )
    assert "Red Hat Lightspeed" in content or "Insights" in content, (
        "Log should contain Lightspeed/Insights connection step"
    )
    assert "yggdrasil" in content, (
        "Log should contain yggdrasil service activation step"
    )


@pytest.mark.tier1
@pytest.mark.parametrize(
    "auth", ["basic", "activation-key"],
)
def test_log_no_sensitive_information(
    external_candlepin, rhc, test_config, log_monitor, auth
):
    """
    :id: b1fbe6f7-754a-44de-be63-c87f1936a077
    :title: Verify no sensitive information logged for both basic and activation-key auth
    :parametrized: yes
    :description:
        Verifies that when connecting with either basic auth (username/password)
        or activation-key auth, the sensitive credential values do not appear
        in the log file.
    :tags: Tier 1
    :steps:
        1. Ensure the system is disconnected.
        2. Run 'rhc connect' using the specified auth method.
        3. Read new log entries.
        4. Verify that sensitive values (password or activation key) are not in the log.
    :expectedresults:
        1. The system is disconnected.
        2. 'rhc connect' executes.
        3. New log entries are captured.
        4. The password or activation key value does not appear in the log file.
    """
    if "satellite" in test_config.environment and auth == "basic":
        pytest.skip("rhc+satellite only support activation key registration now")

    with contextlib.suppress(Exception):
        rhc.disconnect()

    command_args = prepare_args_for_connect(test_config, auth=auth)
    command = ["connect"] + command_args
    rhc.run(*command)

    content = log_monitor.get_new_content()

    if auth == "basic":
        password = test_config.get("candlepin.password")
        assert password not in content, (
            "Password should NOT appear in the log file"
        )
    elif auth == "activation-key":
        activation_key = test_config.get("candlepin.activation_keys")[0]
        assert activation_key not in content, (
            "Activation key should NOT appear in the log file"
        )

