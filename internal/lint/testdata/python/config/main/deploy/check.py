"""Checks of the deploy step."""


def check(rows: list[str]) -> int:
    """Answer the number of rows after asserting there is one."""
    assert rows
    return len(rows)
