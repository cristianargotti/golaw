"""Archive expansion of the fixture script."""

import tarfile


def unpack(archive: str, target: str) -> None:
    """Expand every member of the archive into the target directory."""
    with tarfile.open(archive) as tar:
        for member in tar.getmembers():
            tar.extract(member, target, filter="data")
