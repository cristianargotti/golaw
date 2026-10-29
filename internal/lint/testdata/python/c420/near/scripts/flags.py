"""Flags of the metric script."""


def flags(names: tuple[str, ...]) -> dict[str, None]:
    """Answer every flag unset."""
    return dict.fromkeys(names)


def widths(names: tuple[str, ...]) -> dict[str, int]:
    """Answer the width of every flag."""
    return {name: len(name) for name in names}
