import subprocess


def run(cmd, shell=True, cwd=None):
    """
    Run a command.
    Return exitcode, stdout, stderr
    """

    proc = subprocess.Popen(
        cmd,
        shell=shell,
        cwd=cwd,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        universal_newlines=True,
        errors="surrogateescape",
    )

    stdout, stderr = proc.communicate()
    return proc.returncode, stdout, stderr


def run_in_context(context, cmd, can_fail=False, expected_exit_code=None, **run_args):
    """
    Run a command in the context of a behave scenario.
    :param context: behave context
    :param cmd: command to run
    :param can_fail: whether the command can fail without raising an error
    :param expected_exit_code: expected exit code of the command
    :param run_args: additional arguments to pass to subprocess.Popen
    :return: None
    """

    context.cmd = cmd

    if hasattr(context.scenario, "working_dir") and "cwd" not in run_args:
        run_args["cwd"] = context.scenario.working_dir

    context.cmd_exitcode, context.cmd_stdout, context.cmd_stderr = run(cmd, **run_args)

    if expected_exit_code is not None:
        if expected_exit_code != context.cmd_exitcode:
            raise AssertionError(
                f'Running command "{cmd}" had unexpected exit code: {context.cmd_exitcode}\n'
                f'stdout: {context.cmd_stdout}\nstderr: {context.cmd_stderr}'
            )
    elif not can_fail and context.cmd_exitcode != 0:
        raise AssertionError(
            f'Running command "{cmd}" failed: {context.cmd_exitcode}\n'
            f'stdout: {context.cmd_stdout}\nstderr: {context.cmd_stderr}'
        )
