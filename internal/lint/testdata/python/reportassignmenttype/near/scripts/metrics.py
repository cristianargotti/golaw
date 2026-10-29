"""Metric rows of the report."""


def split_row() -> tuple[str, str]:
    """Answer the name and the state of a metric row."""
    first, second, _ = ("cpu", "busy", "90")
    return first, second
