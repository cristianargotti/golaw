"""Install checks of the deploy scripts."""

import secrets

from deploy import install


def test_install_accepts_the_admin() -> None:
    secret = secrets.token_urlsafe(16)
    assert install.login("admin", secret)


def test_install_sends_the_dsn() -> None:
    dsn = "postgres://admin@db.internal/app"
    assert install.connect(dsn)


def test_flags_are_known() -> None:
    flags = dict.fromkeys(("verbose", "dry_run"))
    assert install.known(flags)


def test_no_flag_is_known() -> None:
    assert install.known({})
