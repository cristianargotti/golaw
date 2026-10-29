"""User lookups of the report."""

import sqlite3


def user_name(connection: sqlite3.Connection, user_id: int) -> object:
    """Answer the name of a user."""
    return connection.execute(
        "SELECT name FROM users WHERE id = ?", (user_id,)
    ).fetchone()
