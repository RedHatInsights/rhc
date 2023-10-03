import pytest

def test_register(subman,settings):
    assert not subman.is_registered
    subman.register(
        username=settings.get("rhsm.account.username"),
        password=settings.get("rhsm.account.password"),
        org=settings.get("rhsm.account.org")
    )
    assert subman.is_registered
    subman.unregister()
    assert not subman.is_registered
