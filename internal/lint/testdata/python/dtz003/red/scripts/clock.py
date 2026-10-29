"""Timestamps of the report."""

import datetime


def stamp() -> datetime.datetime:
    """Answer the time of the report."""
    return datetime.datetime.utcnow()
