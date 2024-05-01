def test_version(rhc):
    proc = rhc.run("--version")
    assert "rhc version " in proc.stdout
