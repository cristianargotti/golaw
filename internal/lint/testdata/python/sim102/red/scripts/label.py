"""Labels of the report."""


def label(score: int, *, strict: bool) -> str:
    """Answer the label of a score."""
    if strict:
        if score > 90:
            return "high"
    return "low"
