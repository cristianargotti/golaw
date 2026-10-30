"""Options of the smoke script."""


def options() -> dict[str, bool]:
    """Answer the default options."""
    return {"verbose": True, "dry_run": False}
