"""Install checks of the deploy scripts."""

import os
import secrets

from deploy import install


def test_install_accepts_the_admin():
    secret = secrets.token_urlsafe(16)
    assert install.login("admin", secret)


def test_install_reads_the_environment():
    env = "POSTGRES_USER=" + os.environ["POSTGRES_USER"]
    assert install.configure(env)
