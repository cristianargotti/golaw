"""Host commands of the deploy script."""

import os


def run(command: str) -> int:
    """Run the command on the host."""
    return os.system(command)
