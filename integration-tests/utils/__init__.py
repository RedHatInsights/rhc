import pytest
import sh
import time


def rhcd_service_is_active(wait_for_stable_state=True, timeout=10, poll_interval=1):
    """Method to verify if rhcd is in active/inactive state
    
    Args:
        wait_for_stable_state (bool): If True, wait for service state to stabilize
        timeout (int): Maximum time to wait for stable state in seconds
        poll_interval (float): Time between state checks in seconds
    
    :return: True if rhcd in active state else False
    """
    def _check_service_state():
        try:
            stdout = sh.systemctl("is-active rhcd".split()).strip()
            return stdout == "active"
        except sh.ErrorReturnCode_3:
            return False
    
    if not wait_for_stable_state:
        return _check_service_state()
    
    # Wait for service state to stabilize by checking if it remains
    # consistent for a short period
    start_time = time.time()
    last_state = None
    stable_since = None
    
    while time.time() - start_time < timeout:
        current_state = _check_service_state()
        
        if current_state == last_state:
            if stable_since is None:
                stable_since = time.time()
            elif time.time() - stable_since >= 1.0:  # State stable for 1 second
                return current_state
        else:
            stable_since = None
            last_state = current_state
        
        time.sleep(poll_interval)
    
    # If we timeout, return the last known state
    return _check_service_state()


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


def prepare_args_for_connect(
    test_config,
    auth: str = None,
    credentials: dict = None,
    content_template: str = None,
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

    if content_template:
        args.extend(["--content-template", content_template])

    return args
