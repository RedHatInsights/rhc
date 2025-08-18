import os
import re
import stat
import subprocess
import pytest
import sh
from contextlib import suppress


def yggdrasil_service_is_active():
    """Method to verify if yggdrasil/rhcd is in active/inactive state
    :return: True if yggdrasil/rhcd in active state else False
    Note: upstream name of service is yggdrasil and downstream is rhcd
    """
    try:
        stdout = sh.systemctl(f"is-active {pytest.service_name}".split()).strip()
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
    Note: upstream name of service is yggdrasil and downstream is rhcd
    """
    if since_datetime:
        logs = sh.journalctl("-u", pytest.service_name, "--since", since_datetime)
    else:
        logs = sh.journalctl("-u", pytest.service_name)

    if must_exist_in_log:
        return str_to_check in logs
    else:
        return str_to_check not in logs


def prepare_args_for_connect(
    test_config, auth: str = None, credentials: dict = None, output_format: str = None
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

    return args


def configure_proxy(test_config, auth_proxy=False):
    """
    Configures the system to use proxy settings and stage server.

    Steps:
    1. Configure subscription-manager to use proxy
    2. Configure insights-client to use proxy
    3. Configure yggdrasil config.toml to use proxy settings
    4. Set up systemd service environment for proxy
    5. Reload systemd daemon
    """
    try:
        # Get proxy configuration
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

        # Call individual configuration methods
        _configure_rhsm_conf(
            test_config,
            proxy_host,
            proxy_port,
            proxy_user if auth_proxy else None,
            proxy_pass if auth_proxy else None,
            auth_proxy,
        )
        _configure_insights_client(proxy_url)
        _configure_yggdrasil_config(test_config)
        _configure_yggdrasil_systemd_proxy_service(proxy_url, "yggdrasil")

        return True

    except Exception as e:
        return False


def _configure_rhsm_conf(
    test_config, proxy_host, proxy_port, proxy_user, proxy_pass, auth_proxy
):
    """
    Configure subscription-manager (rhsm.conf) with proxy and candlepin settings.
    """
    hostname = test_config.get("candlepin.host")
    baseurl = test_config.get("candlepin.baseurl")

    rhsm_replacements = [
        (r"^hostname =.*", f"hostname = {hostname}"),
        (r"^proxy_hostname =.*", f"proxy_hostname = {proxy_host}"),
        (r"^proxy_port =.*", f"proxy_port = {proxy_port}"),
        (r"^baseurl =.*", f"baseurl = {baseurl}"),
    ]

    if auth_proxy:
        rhsm_replacements.extend(
            [
                (r"^proxy_user =.*", f"proxy_user = {proxy_user}"),
                (r"^proxy_password =.*", f"proxy_password = {proxy_pass}"),
            ]
        )

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

    _configure_file_if_exists("/etc/rhsm/rhsm.conf", rhsm_replacements)


def _configure_insights_client(proxy_url):
    """
    Configure insights-client with proxy settings.
    """
    _configure_file_if_exists(
        "/etc/insights-client/insights-client.conf",
        [(r"^#?proxy=.*", f"proxy={proxy_url}")],
    )


def _configure_yggdrasil_config(test_config):
    """
    Configure yggdrasil/rhc service with stage server settings.
    """
    server = test_config.get("rhc.server")
    _configure_file_if_exists(
        "/etc/yggdrasil/config.toml",
        [(r"^server = .*", f'server = ["{server}"]')],
    )


def _configure_yggdrasil_systemd_proxy_service(proxy_url, service_name):
    """
    Configure yggdrasil systemd service with proxy environment.
    """
    # Create systemd override directory
    override_dir = f"/etc/systemd/system/{service_name}.service.d"
    os.makedirs(override_dir, exist_ok=True)
    override_file = f"{override_dir}/proxy.conf"

    # Create systemd override with proxy environment variables directly
    override_content = f"""[Service]
Environment=HTTPS_PROXY={proxy_url}
Environment=HTTP_PROXY={proxy_url}
"""

    with open(override_file, "w") as f:
        f.write(override_content)

    # Reload systemd daemon
    subprocess.run(["systemctl", "daemon-reload"], check=True)


def _configure_file_if_exists(file_path, replacements):
    """
    Helper function to update configuration files if they exist.
    """
    if os.path.exists(file_path):
        try:
            with open(file_path, "r") as f:
                content = f.read()

            for pattern, replacement in replacements:
                content = re.sub(pattern, replacement, content, flags=re.MULTILINE)

            with open(file_path, "w") as f:
                f.write(content)

            return True

        except Exception as e:
            raise
    else:
        return False
