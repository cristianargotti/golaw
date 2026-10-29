"""Hygiene checks of the repository script."""


def hygiene(lines: list[str]) -> int:
    """Count the lines that break a hygiene rule."""
    problems = 0
    for line in lines:
        if line.startswith("a"):
            problems += 1
        if line.startswith("b"):
            problems += 1
        if line.startswith("c"):
            problems += 1
        if line.startswith("d"):
            problems += 1
        if line.startswith("e"):
            problems += 1
        if line.startswith("f"):
            problems += 1
        if line.startswith("g"):
            problems += 1
        if line.startswith("h"):
            problems += 1
        if line.startswith("i"):
            problems += 1
    return problems
