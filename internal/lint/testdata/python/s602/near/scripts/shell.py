"""Commands of the deploy script."""

import subprocess


def status() -> None:
    """Run the status command."""
    subprocess.run(["git", "status"], check=True)
