"""Sampling of the metric script."""

import secrets


def sample(rows: list[str]) -> str:
    """Answer one row."""
    return secrets.choice(rows)
