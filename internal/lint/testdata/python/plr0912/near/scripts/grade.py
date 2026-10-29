"""Grade weights of the report."""


def grade(code: str) -> int:
    """Answer the weight of a grade code."""
    weight = 0
    if code == "a":
        weight += 1
    if code == "b":
        weight += 1
    if code == "c":
        weight += 1
    if code == "d":
        weight += 1
    if code == "e":
        weight += 1
    if code == "f":
        weight += 1
    if code == "g":
        weight += 1
    if code == "h":
        weight += 1
    if code == "i":
        weight += 1
    if code == "j":
        weight += 1
    if code == "k":
        weight += 1
    if code == "l":
        weight += 1
    return weight
