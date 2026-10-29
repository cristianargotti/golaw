"""Host commands of the deploy script."""


class Host:
    """A host the script talks to."""

    def system(self, command: str) -> str:
        """Answer the command the host would run."""
        return command


def run(host: Host, command: str) -> str:
    """Hand the command to the host."""
    return host.system(command)
