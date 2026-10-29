"""Install checks of the deploy step."""

from deploy import install


def test_login() -> None:
    assert install.login("admin", "role")
