"""
:casecomponent: rhc
:requirement: RHSS-291300
:polarion-project-id: RHELSS
:polarion-include-skipped: false
:polarion-lookup-method: id
:subsystemteam: rhel-sst-csi-client-tools
:caseautomation: Automated
:upstream: Yes
"""

def test_version(rhc):
    """
    :id: 87c8472e-3f3c-4481-ae3b-e7511420feaf
    :title: Verify rhc version command output
    :description:
        This test verifies that the `rhc --version` command executes successfully
        and outputs a string containing the expected version prefix.
    :tags:
    :steps:
        1.  Run the `rhc --version` command.
    :expectedresults:
        1.  The standard output should contain the string "rhc version ".
    """

    proc = rhc.run("--version")
    assert "rhc version " in proc.stdout
