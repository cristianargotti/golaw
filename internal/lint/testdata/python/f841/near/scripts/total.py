"""Totals of the report."""


def total(rows: list[int]) -> int:
    """Answer the sum of the rows, zero for none."""
    count = len(rows)
    return sum(rows) if count else 0
