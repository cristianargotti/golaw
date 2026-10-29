"""Load the rules the deploy step reads."""

import ast


def load(text: str) -> object:
    """Answer the rule the text spells; eval(text) would run it."""
    return ast.literal_eval(text)
