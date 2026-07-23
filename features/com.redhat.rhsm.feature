Feature: The Varlink interface com.redhat.rhsm
  The basic Varlink interface com.redhat.rhsm provides basic
  methods that allows to check if the system is registered and
  if the system can reach candlepin server


  Scenario: IsRegistered() method returns false on unregistered system
    Given system is not registered
    When varlink method is called
      | method       | interface               | arguments |
      | IsRegistered | com.redhat.rhsm.testing | '{}'      |
    Then varlink method returns
      """
      {"registered":false}
      """

  Scenario: Ping() method gets status of candlepin server on unregistered system
    Given system is not registered
    When varlink method is called
      | method | interface               | arguments |
      | Ping   | com.redhat.rhsm.testing | '{}'      |
    Then method call was successful
    And method returned JSON compliant with 'com.redhat.rhsm.testing.Ping.json' schema


  Scenario: IsRegistered() method returns true on registered system
    Given system is registered against candlepin server
    When varlink method is called
      | method       | interface               | arguments |
      | IsRegistered | com.redhat.rhsm.testing | '{}'      |
    Then varlink method returns
      """
      {"registered":true}
      """

  Scenario: Ping() method gets status of candlepin server on registered system
    Given system is registered against candlepin server
    When varlink method is called
      | method | interface               | arguments |
      | Ping   | com.redhat.rhsm.testing | '{}'      |
    Then method call was successful
    And method returned JSON compliant with 'com.redhat.rhsm.testing.Ping.json' schema


  Scenario: Ping() method gets status of candlepin server on registered system with metadata
    Given system is registered against candlepin server
    When varlink method is called
      | method | interface               | arguments          |
      | Ping   | com.redhat.rhsm.testing | '{"metadata": {}}' |
    Then method call was successful
    And method returned JSON compliant with 'com.redhat.rhsm.testing.Ping.json' schema


  Scenario: Ping() method gets status of candlepin server on registered system with metadata (user_agent)
    Given system is registered against candlepin server
    When varlink method is called
      | method | interface               | arguments                             |
      | Ping   | com.redhat.rhsm.testing | '{"metadata": {"user_agent": "foo"}}' |
    Then method call was successful
    And method returned JSON compliant with 'com.redhat.rhsm.testing.Ping.json' schema


  Scenario: Ping() method gets status of candlepin server on registered system with metadata (correlation ID)
    Given system is registered against candlepin server
    When varlink method is called
      | method | interface               | arguments                                                                  |
      | Ping   | com.redhat.rhsm.testing | '{"metadata": {"correlation_id": "670fcfe2-d87c-40f1-8ea9-1bfd17664fe4"}}' |
    Then method call was successful
    And method returned JSON compliant with 'com.redhat.rhsm.testing.Ping.json' schema


  Scenario: Ping() method gets status of candlepin server on registered system with metadata (locale)
    Given system is registered against candlepin server
    When varlink method is called
      | method | interface               | arguments                           |
      | Ping   | com.redhat.rhsm.testing | '{"metadata": {"locale": "de_DE"}}' |
    Then method call was successful
    And method returned JSON compliant with 'com.redhat.rhsm.testing.Ping.json' schema
