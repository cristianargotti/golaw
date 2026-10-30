"""Commands of the deploy script."""

import subprocess


def status(command: str) -> None:
    """Run the status command."""
    subprocess.run(command, shell=True, check=True)
