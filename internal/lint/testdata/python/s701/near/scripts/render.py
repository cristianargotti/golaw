"""Templates of the report."""

import jinja2


def environment(loader: jinja2.BaseLoader) -> jinja2.Environment:
    """Answer the template environment."""
    return jinja2.Environment(loader=loader, autoescape=True)
