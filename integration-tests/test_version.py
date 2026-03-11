import pytest


@pytest.mark.tier1
def test_version(rhc):
    proc = rhc.run("--version")
    assert "rhc version " in proc.stdout
