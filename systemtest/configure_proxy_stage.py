#!/usr/bin/env python3
"""Configure system-wide proxy settings when ENV_FOR_DYNACONF is 'stage'.

Uses local settings.toml if present, otherwise downloads from SETTINGS_URL.
"""

import configparser
import os
import subprocess
import sys
import urllib.error
import urllib.request
from pathlib import Path

try:
    import tomllib  # Python 3.11+
except ModuleNotFoundError:
    import tomli as tomllib


ENV_VAR = "ENV_FOR_DYNACONF"
TARGET_ENV = "stage"

RHSM_CONF = Path("/etc/rhsm/rhsm.conf")
INSIGHTS_CONF = Path("/etc/insights-client/insights-client.conf")
RHCD_OVERRIDE_DIR = Path("/etc/systemd/system/rhcd.service.d")
RHCD_OVERRIDE = RHCD_OVERRIDE_DIR / "proxy.conf"
PROFILE_PROXY = Path("/etc/profile.d/proxy.sh")
LOCAL_SETTINGS = Path(__file__).parent.parent / "settings.toml"


def load_proxy_settings() -> tuple[str, str, str]:
    """Load noauth_proxy host/port from local settings.toml or SETTINGS_URL."""
    if LOCAL_SETTINGS.exists():
        with LOCAL_SETTINGS.open("rb") as fh:
            data = tomllib.load(fh)
    else:
        settings_url = os.environ.get("SETTINGS_URL")
        if not settings_url:
            raise RuntimeError(
                f"No local settings.toml found at {LOCAL_SETTINGS} and SETTINGS_URL not set"
            )
        try:
            with urllib.request.urlopen(settings_url) as response:
                data = tomllib.load(response)
        except (urllib.error.URLError, urllib.error.HTTPError) as exc:
            raise RuntimeError(f"Failed to download settings: {exc}")

    stage_config = data.get(TARGET_ENV)
    if not stage_config:
        raise ValueError(f"'{TARGET_ENV}' config section not found in settings")

    noauth_proxy = stage_config.get("noauth_proxy")
    if not noauth_proxy:
        raise ValueError(f"'noauth_proxy' not found in '{TARGET_ENV}' section")

    host = noauth_proxy.get("host")
    port = noauth_proxy.get("port")
    if not host or not port:
        raise ValueError("host/port not found in noauth_proxy")

    proxy_url = f"http://{host}:{port}"
    return host, str(port), proxy_url


def update_rhsm_conf(host: str, port: str):
    """Update /etc/rhsm/rhsm.conf with proxy settings."""
    if not RHSM_CONF.exists():
        print(f"Skipping RHSM proxy configuration: {RHSM_CONF} does not exist")
        return

    parser = configparser.ConfigParser()
    parser.optionxform = str  # preserve case
    parser.read(RHSM_CONF)

    if "server" not in parser:
        parser["server"] = {}

    parser["server"]["proxy_hostname"] = host
    parser["server"]["proxy_port"] = port

    with RHSM_CONF.open("w") as fh:
        parser.write(fh)


def update_insights_conf(proxy_url: str):
    """Update /etc/insights-client/insights-client.conf with proxy setting."""
    if not INSIGHTS_CONF.exists():
        print(f"Skipping Insights proxy configuration: {INSIGHTS_CONF} does not exist")
        return

    parser = configparser.ConfigParser()
    parser.optionxform = str  # preserve case
    parser.read(INSIGHTS_CONF)

    if "insights-client" not in parser:
        parser["insights-client"] = {}

    parser["insights-client"]["proxy"] = proxy_url

    with INSIGHTS_CONF.open("w") as fh:
        parser.write(fh)


def configure_rhcd_service(proxy_url: str):
    """Create systemd override for rhcd with proxy environment variables."""
    RHCD_OVERRIDE_DIR.mkdir(parents=True, exist_ok=True)

    override = f"""[Service]
Environment=HTTPS_PROXY={proxy_url}
Environment=HTTP_PROXY={proxy_url}
"""
    RHCD_OVERRIDE.write_text(override)
    subprocess.run(["systemctl", "daemon-reload"], check=True)


def configure_profile_proxy(proxy_url: str):
    """Add proxy env vars to /etc/profile.d for future shell sessions."""
    content = f"""export HTTPS_PROXY={proxy_url}
export HTTP_PROXY={proxy_url}
"""
    PROFILE_PROXY.write_text(content)


def main() -> int:
    env_value = os.environ.get(ENV_VAR, "").lower()
    if env_value != TARGET_ENV:
        print(
            f"{ENV_VAR} is '{env_value or 'unset'}'; skipping proxy configuration "
            f"(expected '{TARGET_ENV}')."
        )
        return 0

    try:
        host, port, proxy_url = load_proxy_settings()
        update_rhsm_conf(host, port)
        update_insights_conf(proxy_url)
        configure_rhcd_service(proxy_url)
        configure_profile_proxy(proxy_url)
    except Exception as exc:
        print(f"Failed to configure proxy: {exc}", file=sys.stderr)
        return 1

    print("Proxy configuration applied for stage environment.")
    return 0


if __name__ == "__main__":
    sys.exit(main())
