"""Load the rules the deploy step reads."""


def load(text: str) -> object:
    """Answer the rule the text spells."""
    return eval(text)
