"""Flags of the metric script."""


def flags(names: tuple[str, ...]) -> dict[str, None]:
    """Answer every flag unset."""
    return {name: None for name in names}
