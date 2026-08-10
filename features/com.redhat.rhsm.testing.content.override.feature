Feature: The Varlink interface com.redhat.rhsm.testing.content.override
  The Varlink interface com.redhat.rhsm.testing.content.override provides
  methods for downloading content overrides from the candlepin server and
  uploading local DNF5 repo overrides to the server.


  Scenario: Download() method raises error on unregistered system
    Given system is not registered
    When varlink method is called and error is expected
      | method   | interface                                | arguments |
      | Download | com.redhat.rhsm.testing.content.override | '{}'      |
    Then varlink error is raised
      """
      com.redhat.rhsm.testing.content.override.SystemNotRegistered
      """


  Scenario: Upload() method raises error on unregistered system
    Given system is not registered
    When varlink method is called and error is expected
      | method | interface                                | arguments |
      | Upload | com.redhat.rhsm.testing.content.override | '{}'      |
    Then varlink error is raised
      """
      com.redhat.rhsm.testing.content.override.SystemNotRegistered
      """


  Scenario: Download() method returns content overrides on registered system
    Given system is registered against candlepin server
    When varlink method is called
      | method   | interface                                | arguments |
      | Download | com.redhat.rhsm.testing.content.override | '{}'      |
    Then method call was successful
    And method returned JSON compliant with 'com.redhat.rhsm.testing.content.override.Download.json' schema


  Scenario: Download() method returns content overrides on registered system with metadata
    Given system is registered against candlepin server
    When varlink method is called
      | method   | interface                                | arguments                             |
      | Download | com.redhat.rhsm.testing.content.override | '{"metadata": {"user_agent": "foo"}}' |
    Then method call was successful
    And method returned JSON compliant with 'com.redhat.rhsm.testing.content.override.Download.json' schema


  @fixture.no_redhat_dnf5_override_installed
  Scenario: Upload() method uploads local overrides on registered system
    Given system is registered against candlepin server
    And local DNF5 repo override file exists with content
      """
      [test-override-repo]
      enabled = 1
      """
    When varlink method is called
      | method | interface                                | arguments |
      | Upload | com.redhat.rhsm.testing.content.override | '{}'      |
    Then varlink method returns
      """
      {"success":true}
      """


  @fixture.no_redhat_dnf5_override_installed
  Scenario: Upload() then Download() round-trip preserves overrides
    Given system is registered against candlepin server
    And local DNF5 repo override file exists with content
      """
      [test-roundtrip-repo]
      enabled = 1
      gpgcheck = 0
      """
    When varlink method is called
      | method | interface                                | arguments |
      | Upload | com.redhat.rhsm.testing.content.override | '{}'      |
    Then method call was successful
    When varlink method is called
      | method   | interface                                | arguments |
      | Download | com.redhat.rhsm.testing.content.override | '{}'      |
    Then method call was successful
    And downloaded content overrides contain label 'test-roundtrip-repo'


  @fixture.no_redhat_dnf5_override_installed
  Scenario: Upload() succeeds with empty override file on registered system
    Given system is registered against candlepin server
    And local DNF5 repo override file is empty
    When varlink method is called
      | method | interface                                | arguments |
      | Upload | com.redhat.rhsm.testing.content.override | '{}'      |
    Then varlink method returns
      """
      {"success":true}
      """


  Scenario: Download() returns consistent results across multiple calls
    Given system is registered against candlepin server
    When varlink method is called
      | method   | interface                                | arguments |
      | Download | com.redhat.rhsm.testing.content.override | '{}'      |
    Then method call was successful
    And method result is saved as 'first_download'
    When varlink method is called
      | method   | interface                                | arguments |
      | Download | com.redhat.rhsm.testing.content.override | '{}'      |
    Then method call was successful
    And method result matches saved 'first_download'
