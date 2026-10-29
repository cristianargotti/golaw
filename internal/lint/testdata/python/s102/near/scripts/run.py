"""Steps of the smoke script."""


class Runner:
    """Run a step."""

    def exec(self, source: str) -> str:
        """Answer the step source."""
        return source


def run(runner: Runner, source: str) -> str:
    """Hand the step to the runner."""
    return runner.exec(source)
