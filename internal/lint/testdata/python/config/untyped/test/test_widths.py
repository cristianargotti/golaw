"""Width checks of the report."""


def width(text):
    return len(text)


def test_width() -> None:
    assert width("cpu") == 3
