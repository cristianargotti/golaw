"""Name lengths of the report."""


def lengths(names: list[str]) -> dict[str, int]:
    """Answer the length of every name."""
    return dict((name, len(name)) for name in names)
