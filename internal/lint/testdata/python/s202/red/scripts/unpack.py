"""Archive expansion of the fixture script."""

import tarfile


def unpack(archive: str, target: str) -> None:
    """Expand the archive into the target directory."""
    with tarfile.open(archive) as tar:
        tar.extractall(target)
