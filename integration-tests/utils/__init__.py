import pytest
import sh


def rhcd_service_is_active():
    """Method to verify if rhcd is in active/inactive state
    :return: True if rhcd in active state else False
    """
    try:
        stdout = sh.systemctl("is-active rhcd".split()).strip()
        return stdout == "active"
    except sh.ErrorReturnCode_3:
        return False


def check_rhcd_journalctl_logs(
    str_to_check, since_datetime=None, must_exist_in_log=True
):
    """This method helps in verifying strings in rhcd journalctl logs
    :param str_to_check: string to be searched in logs
    :param since_datetime: start time for logs
    :param must_exist_in_log: True if str_to_check should exist in log else false
    :return: True/False
    """
    if since_datetime:
        logs = sh.journalctl("-u", "rhcd", "--since", since_datetime)
    else:
        logs = sh.journalctl("-u", "rhcd")

    if must_exist_in_log:
        return str_to_check in logs
    else:
        return str_to_check not in logs


def prepare_args_for_connect(test_config, auth: str = None, credentials: dict = None):
    """Method to create arguments to be passed in 'rhc connect' command
    This method expects either auth type or custom credentials
    """
    args = []
    if credentials:
        for k, v in credentials.items():
            try:
                value = test_config.get(v)
                value = value[0] if isinstance(value, list) else value
            except KeyError:
                value = v
            if value:
                args.extend([f"--{k}", value])

    elif auth:
        if auth == "basic":
            args.extend(
                [
                    "--username",
                    test_config.get("candlepin.username"),
                    "--password",
                    test_config.get("candlepin.password"),
                ]
            )

        elif auth == "activation-key":
            args.extend(
                [
                    "--activation-key",
                    test_config.get("candlepin.activation_keys")[0],
                    "--organization",
                    test_config.get("candlepin.org"),
                ]
            )

    return args
