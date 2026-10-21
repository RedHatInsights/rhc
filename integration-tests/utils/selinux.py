import subprocess


def _get_selinux_mode():
    """Get current SELinux mode (Enforcing, Permissive, or Disabled)."""
    try:
        result = subprocess.run(["getenforce"], capture_output=True, text=True, check=True)
        return result.stdout.strip()
    except (subprocess.CalledProcessError, FileNotFoundError):
        return "Disabled"


def is_selinux_disabled():
    """Check if SELinux is disabled."""
    return _get_selinux_mode() == "Disabled"


def label_type(path):
    """Get the type of a SELinux label for a given path."""
    context = subprocess.check_output(["stat", "-c", "%C", path], text=True).strip()
    context_parts = context.split(":")
    if len(context_parts) < 4:
        raise AssertionError(f"{path} has no SELinux type in context '{context}'")
    return context_parts[2]
