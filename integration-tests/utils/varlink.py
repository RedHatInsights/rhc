"""
Varlink-related utility functions for integration tests.

This module provides helper functions for interacting with varlink services
using varlinkctl commands.
"""

import subprocess
import json
from typing import Optional, Union


def run_varlinkctl(
    method: str,
    params: Optional[dict] = None,
    check: bool = True
) -> Union[dict, subprocess.CompletedProcess]:
    """
    Helper function to call varlinkctl and return a parsed JSON response.

    :param method: The varlink method to call (e.g., "com.redhat.rhc.collector.List")
    :param params: Optional parameters as a dictionary
    :param check: If True, raise exception on non-zero exit code (default: True)
    :return: Parsed JSON response if check=True, otherwise CompletedProcess object
    """
    cmd = ["varlinkctl", "call", "unix:/run/rhc/com.redhat.rhc", method]

    if params:
        cmd.append(json.dumps(params))
    else:
        cmd.append("{}")

    result = subprocess.run(cmd, capture_output=True, text=True, check=check)

    if check:
        return json.loads(result.stdout)
    else:
        return result
