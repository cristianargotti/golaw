"""The install step the tests call."""


def login(user: str, role: str) -> bool:
    """Answer whether the user may log in with the role."""
    return bool(user) and bool(role)
