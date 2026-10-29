"""Digests of the release script."""

import hashlib


def digest(data: bytes) -> str:
    """Answer the digest of the data."""
    return hashlib.md5(data).hexdigest()
