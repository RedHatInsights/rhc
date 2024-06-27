import sh


def yggdrasil_service_is_active():
    """Method to verify if yggdrasil is in active/inactive state
    :return: True if yggdrasil in active state else False
    """
    try:
        stdout = sh.systemctl("is-active yggdrasil".split()).strip()
        return stdout == "active"
    except sh.ErrorReturnCode_3:
        return False


def check_yggdrasil_journalctl(
    str_to_check, since_datetime=None, must_exist_in_log=True
):
    """This method helps in verifying strings in yggdrasil logs
    :param str_to_check: string to be searched in logs
    :param since_datetime: start time for logs
    :param must_exist_in_log: True if str_to_check should exist in log else false
    :return: True/False
    """
    if since_datetime:
        yggdrasil_logs = sh.journalctl("-u", "yggdrasil", "--since", since_datetime)
    else:
        yggdrasil_logs = sh.journalctl("-u", "yggdrasil")

    if must_exist_in_log:
        return str_to_check in yggdrasil_logs
    else:
        return str_to_check not in yggdrasil_logs


def prepare_args_for_connect(
    test_config, auth: str = None, credentials: dict = None, server: str = None
):
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

    if server:
        args.extend(["--server", server])

    return args
