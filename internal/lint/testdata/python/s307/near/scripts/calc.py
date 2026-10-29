"""Arithmetic of the report script."""

import ast


def total(expression: str) -> int:
    """Answer the value of the literal expression."""
    return ast.literal_eval(expression)
