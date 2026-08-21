Feature: The Varlink interface com.redhat.rhsm.testing.content
  The Varlink interface com.redhat.rhsm.testing.content provides a method
  for refreshing the installed SCA entitlement certificate and regenerating
  the redhat.repo file accordingly.


  # Scenarios for unregistered systems first

  Scenario: Refresh() method raises error on unregistered system
    Given system is not registered
    When varlink method is called and error is expected
      | method  | interface                       | arguments          |
      | Refresh | com.redhat.rhsm.testing.content | '{"force": false}' |
    Then varlink error is raised
      """
      com.redhat.rhsm.testing.content.SystemNotRegistered
      """


  # Scenarios for registered systems

  Scenario: Refresh() method succeeds on registered system
    Given system is registered against candlepin server
    When varlink method is called
      | method  | interface                       | arguments          |
      | Refresh | com.redhat.rhsm.testing.content | '{"force": false}' |
    Then varlink method returns
      """
      {"success":true}
      """


  Scenario: Refresh() method succeeds on registered system with metadata
    Given system is registered against candlepin server
    When varlink method is called
      | method  | interface                       | arguments                                             |
      | Refresh | com.redhat.rhsm.testing.content | '{"force": false, "metadata": {"user_agent": "foo"}}' |
    Then varlink method returns
      """
      {"success":true}
      """


  Scenario: Refresh(force=true) method succeeds on registered system
    Given system is registered against candlepin server
    When varlink method is called
      | method  | interface                       | arguments         |
      | Refresh | com.redhat.rhsm.testing.content | '{"force": true}' |
    Then varlink method returns
      """
      {"success":true}
      """


  Scenario: Refresh() called twice on a registered system succeeds both times
    Given system is registered against candlepin server
    When varlink method is called
      | method  | interface                       | arguments          |
      | Refresh | com.redhat.rhsm.testing.content | '{"force": false}' |
    Then varlink method returns
      """
      {"success":true}
      """
    And varlink method is called
      | method  | interface                       | arguments          |
      | Refresh | com.redhat.rhsm.testing.content | '{"force": false}' |
    Then varlink method returns
      """
      {"success":true}
      """
