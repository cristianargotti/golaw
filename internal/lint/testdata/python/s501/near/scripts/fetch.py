"""Health probe of the smoke script."""

import requests


def fetch(url: str) -> int:
    """Answer the status of the url."""
    return requests.get(url, verify=True, timeout=10).status_code
