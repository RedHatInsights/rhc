#!/usr/bin/env python3

# Configure system-wide proxy settings when Dynaconf environment is "stage".
# This script expects a Dynaconf-style `settings.toml` downloaded via
# `SETTINGS_URL`, with a `[stage]` section containing `noauth_proxy.host`
# and `noauth_proxy.port`.

import configparser
import os
import subprocess
import sys
import tempfile
import urllib.error
import urllib.request
from pathlib import Path

try:
    import tomllib  # Python 3.11+
except ModuleNotFoundError:  # pragma: no cover - fallback for older pythons
    import tomli as tomllib


ENV_VAR = "ENV_FOR_DYNACONF"
TARGET_ENV = "stage"
SETTINGS_URL = os.environ.get("SETTINGS_URL")

RHSM_CONF = Path("/etc/rhsm/rhsm.conf")
INSIGHTS_CONF = Path("/etc/insights-client/insights-client.conf")
YGGDRASIL_OVERRIDE_DIR = Path("/etc/systemd/system/yggdrasil.service.d")
YGGDRASIL_OVERRIDE_FILE = YGGDRASIL_OVERRIDE_DIR / "proxy.conf"


def resolve_settings_file() -> Path:
    """
    Resolve the settings source:
    - SETTINGS_URL must be provided; download it to a temporary file.
    """
    if not SETTINGS_URL:
        raise RuntimeError("SETTINGS_URL must be provided to configure proxy.")

    tmp_fd, tmp_path = tempfile.mkstemp(prefix="settings_", suffix=".toml")
    os.close(tmp_fd)
    try:
        urllib.request.urlretrieve(SETTINGS_URL, tmp_path)
    except (urllib.error.URLError, urllib.error.HTTPError) as exc:
        raise RuntimeError(f"Failed to download settings from {SETTINGS_URL}: {exc}")
    return Path(tmp_path)


def load_stage_proxy():
    """Load proxy settings only from the SETTINGS_URL-provided settings file."""
    settings_path = resolve_settings_file()

    with settings_path.open("rb") as fh:
        data = tomllib.load(fh)

    env_config = data.get(TARGET_ENV) or {}
    noauth_cfg = env_config.get("noauth_proxy") or {}

    host = noauth_cfg.get("host")
    port = noauth_cfg.get("port")

    if not host or not port:
        raise ValueError(
            f"Proxy host/port not set for '{TARGET_ENV}' in {settings_path}"
        )

    proxy_url = f"http://{host}:{port}"
    return host, str(port), proxy_url


def update_rhsm_conf(host: str, port: str):
    """Update /etc/rhsm/rhsm.conf with proxy settings."""
    RHSM_CONF.parent.mkdir(parents=True, exist_ok=True)
    parser = configparser.ConfigParser()
    parser.optionxform = str  # keep keys case-sensitive
    parser.read(RHSM_CONF)

    if "server" not in parser:
        parser["server"] = {}

    parser["server"]["proxy_hostname"] = host
    parser["server"]["proxy_port"] = port

    with RHSM_CONF.open("w") as fh:
        parser.write(fh)


def update_insights_conf(proxy_url: str):
    """Update /etc/insights-client/insights-client.conf with proxy setting."""
    INSIGHTS_CONF.parent.mkdir(parents=True, exist_ok=True)
    parser = configparser.ConfigParser()
    parser.optionxform = str
    parser.read(INSIGHTS_CONF)

    section = "insights-client"
    if section not in parser:
        parser[section] = {}

    parser[section]["proxy"] = proxy_url

    with INSIGHTS_CONF.open("w") as fh:
        parser.write(fh)


def configure_yggdrasil_service(proxy_url: str):
    """Create systemd override for yggdrasil with proxy environment variables."""
    YGGDRASIL_OVERRIDE_DIR.mkdir(parents=True, exist_ok=True)
    override = "[Service]\n"
    override += f"Environment=HTTPS_PROXY={proxy_url}\n"
    override += f"Environment=HTTP_PROXY={proxy_url}\n"

    with YGGDRASIL_OVERRIDE_FILE.open("w") as fh:
        fh.write(override)

    subprocess.run(["systemctl", "daemon-reload"], check=True)


def main() -> int:
    env_value = os.environ.get(ENV_VAR, "").lower()
    if env_value != TARGET_ENV:
        print(
            f"{ENV_VAR} is '{env_value or 'unset'}'; skipping proxy configuration "
            f"(expected '{TARGET_ENV}')."
        )
        return 0

    try:
        host, port, proxy_url = load_stage_proxy()
    except Exception as exc:  # pragma: no cover - runtime diagnostic
        print(f"Failed to load proxy settings: {exc}", file=sys.stderr)
        return 1

    try:
        update_rhsm_conf(host, port)
        update_insights_conf(proxy_url)
        configure_yggdrasil_service(proxy_url)
    except Exception as exc:  # pragma: no cover - runtime diagnostic
        print(f"Failed to configure proxy: {exc}", file=sys.stderr)
        return 1

    print("Proxy configuration applied for stage environment.")
    return 0


if __name__ == "__main__":
    sys.exit(main())
