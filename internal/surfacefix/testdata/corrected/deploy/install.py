"""The install steps the deploy tests call."""


def login(user: str, secret: str) -> bool:
    """Answer whether the user may log in with the secret."""
    return bool(user) and bool(secret)


def connect(dsn: str) -> bool:
    """Answer whether the data source name is complete."""
    return "://" in dsn


def known(flags: dict[str, None]) -> bool:
    """Answer whether every flag is known."""
    return set(flags) <= {"verbose", "dry_run"}
