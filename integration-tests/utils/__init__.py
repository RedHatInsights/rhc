import subprocess
import sh
from contextlib import suppress


def yggdrasil_service_is_active():
    """Method to verify if yggdrasil is in active/inactive state
    :return: True if yggdrasil in active state else False
    """
    try:
        stdout = sh.systemctl(f"is-active yggdrasil".split()).strip()
        return stdout == "active"
    except sh.ErrorReturnCode_3:
        return False


def check_yggdrasil_journalctl_logs(
    str_to_check, since_datetime=None, must_exist_in_log=True
):
    """This method helps in verifying strings in journalctl logs
    :param str_to_check: string to be searched in logs
    :param since_datetime: start time for logs
    :param must_exist_in_log: True if str_to_check should exist in log else false
    :return: True/False
    """
    if since_datetime:
        logs = sh.journalctl("-u", "yggdrasil", "--since", since_datetime)
    else:
        logs = sh.journalctl("-u", "yggdrasil")

    if must_exist_in_log:
        return str_to_check in logs
    else:
        return str_to_check not in logs


def prepare_args_for_connect(
    test_config,
    auth: str = None,
    credentials: dict = None,
    output_format: str = None,
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

    if output_format:
        args.extend(["--format", output_format])

    if content_template:
        args.extend(["--content-template", content_template])

    return args


def configure_proxy_rhsm(subman, test_config, auth_proxy=False):
    """
    Configures the system to use proxy settings and stage server.

    Steps:
    1. Configure subscription-manager to use proxy
    2. Configure insights-client to use proxy
    3. Configure yggdrasil config.toml to use intended server(e.g. stage server)
    4. Set up systemd service environment for proxy
    5. Reload systemd daemon
    """
    try:
        service_name = "yggdrasil"

        # Get proxy configuration from settings.toml
        if auth_proxy:
            proxy_host = test_config.get("auth_proxy.host")
            proxy_user = test_config.get("auth_proxy.username")
            proxy_pass = test_config.get("auth_proxy.password")
            proxy_port = str(test_config.get("auth_proxy.port"))
            proxy_url = f"http://{proxy_user}:{proxy_pass}@{proxy_host}:{proxy_port}"
        else:
            proxy_host = test_config.get("noauth_proxy.host")
            proxy_port = str(test_config.get("noauth_proxy.port"))
            proxy_url = f"http://{proxy_host}:{proxy_port}"

        # Configure subscription-manager rhsm.conf
        hostname = test_config.get("candlepin.host")
        baseurl = test_config.get("candlepin.baseurl")

        subman.config(server_hostname=hostname)
        subman.config(rhsm_baseurl=baseurl)
        subman.config(server_proxy_hostname=proxy_host)
        subman.config(server_proxy_port=proxy_port)

        if auth_proxy:
            subman.config(server_proxy_user=proxy_user)
            subman.config(server_proxy_password=proxy_pass)

            # Configure SELinux for auth proxy port
            with suppress(FileNotFoundError, subprocess.CalledProcessError):
                subprocess.run(
                    [
                        "semanage",
                        "port",
                        "-a",
                        "-t",
                        "http_cache_port_t",
                        "-p",
                        "tcp",
                        proxy_port,
                    ],
                    check=True,
                    capture_output=True,
                    text=True,
                )

        print(f"Proxy configuration completed for {service_name}")
        return proxy_url

    except Exception as e:
        print(f"Error during stage configuration: {e}")
        return False
