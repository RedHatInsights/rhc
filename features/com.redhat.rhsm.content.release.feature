Feature: The Varlink interface com.redhat.rhsm.content.release
  The Varlink interface com.redhat.rhsm.content.release provides
  methods that allows to manage release version (releasever) of
  RHEL system

  # Scenarios for unregistered systems first

  Scenario: Download() method raises error on unregistered system
    Given system is not registered
    When varlink method is called and error is expected
      | method   | interface                               | arguments |
      | Download | com.redhat.rhsm.testing.content.release | '{}'      |
    Then varlink error is raised
      """
      com.redhat.rhsm.testing.content.release.SystemNotRegistered
      """

  Scenario: GetAvailableReleases() method raises error on unregistered system
    Given system is not registered
    When varlink method is called and error is expected
      | method               | interface                               | arguments |
      | GetAvailableReleases | com.redhat.rhsm.testing.content.release | '{}'      |
    Then varlink error is raised
      """
      com.redhat.rhsm.testing.content.release.SystemNotRegistered
      """

  Scenario: GetCurrentRelease() method returns empty object on unregistered system (releasever file is empty)
    Given system is not registered
    Given releasever file is empty
    When varlink method is called
      | method            | interface                               | arguments |
      | GetCurrentRelease | com.redhat.rhsm.testing.content.release | '{}'      |
    Then varlink method returns
      """
      {}
      """

  Scenario: GetCurrentRelease() method returns empty object on unregistered system (releasever does not exist)
    Given system is not registered
    Given releasever file is deleted
    When varlink method is called
      | method            | interface                               | arguments |
      | GetCurrentRelease | com.redhat.rhsm.testing.content.release | '{}'      |
    Then varlink method returns
      """
      {}
      """

  Scenario: GetCurrentRelease() method returns content of releasever file
    Given system is not registered
    Given releasever file contains
      """
      44
      """
    When varlink method is called
      | method            | interface                               | arguments |
      | GetCurrentRelease | com.redhat.rhsm.testing.content.release | '{}'      |
    Then varlink method returns
      """
      {"release":"44"}
      """

  Scenario: SetRelease() method set release on unregistered system
    Given system is not registered
    Given releasever file is deleted
    When varlink method is called
      | method     | interface                               | arguments                |
      | SetRelease | com.redhat.rhsm.testing.content.release | '{"Release": "44"}'      |
    Then varlink method returns
      """
      {"success":true}
      """
    And releasever file contains expected content
      """
      44
      """
    And varlink method is called
      | method       | interface                               | arguments |
      | UnsetRelease | com.redhat.rhsm.testing.content.release | '{}'      |

  Scenario: SetRelease({"Release": ""}) method unset release on unregistered systems
    Given system is not registered
    Given releasever file contains
      """
      44
      """
    When varlink method is called
      | method     | interface                               | arguments              |
      | SetRelease | com.redhat.rhsm.testing.content.release | '{"Release": ""}'      |
    Then varlink method returns
      """
      {"success":true}
      """
    And releasever file does not exists

  Scenario: UnsetRelease() method unset release on unregistered systems
    Given system is not registered
    Given releasever file contains
      """
      44
      """
    When varlink method is called
      | method       | interface                               | arguments |
      | UnsetRelease | com.redhat.rhsm.testing.content.release | '{}'      |
    Then varlink method returns
      """
      {"success":true}
      """
    And releasever file does not exists

  # Scenarios for registered systems. Given organization does not contain any product with release

  Scenario: Download() method returns empty release
    Given system is registered against candlepin server
    When varlink method is called
      | method   | interface                               | arguments |
      | Download | com.redhat.rhsm.testing.content.release | '{}'      |
    Then varlink method returns
      """
      {"release":""}
      """

  Scenario: GetAvailableReleases() method returns empty list on registered system
    Given system is registered against candlepin server
    When varlink method is called
      | method               | interface                               | arguments |
      | GetAvailableReleases | com.redhat.rhsm.testing.content.release | '{}'      |
    Then varlink method returns
      """
      {"releases":[]}
      """

  Scenario: SetRelease() method set release and UnsetRelease() unset the release on registered system
    Given system is registered against candlepin server
    Given releasever file is deleted
    When varlink method is called
      | method     | interface                               | arguments                |
      | SetRelease | com.redhat.rhsm.testing.content.release | '{"Release": "44"}'      |
    Then varlink method returns
      """
      {"success":true}
      """
    And releasever file contains expected content
      """
      44
      """
    And wait '0.1' seconds
    And varlink method is called
      | method   | interface                               | arguments |
      | Download | com.redhat.rhsm.testing.content.release | '{}'      |
    Then varlink method returns
      """
      {"release":"44"}
      """
    And varlink method is called
      | method       | interface                               | arguments |
      | UnsetRelease | com.redhat.rhsm.testing.content.release | '{}'      |
    Then varlink method returns
      """
      {"success":true}
      """
    And releasever file does not exists
    And wait '0.1' seconds
    And varlink method is called
      | method   | interface                               | arguments |
      | Download | com.redhat.rhsm.testing.content.release | '{}'      |
    Then varlink method returns
      """
      {"release":""}
      """
