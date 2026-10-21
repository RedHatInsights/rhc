Feature: The Varlink interface com.redhat.rhsm.testing.content.profile
  The Varlink interface com.redhat.rhsm.testing.content.profile provides a
  method for gathering the combined package profile (the list of installed
  RPM packages and the list of enabled repositories) and uploading it to
  the candlepin server. The candlepin server has to support the combined
  profile upload, otherwise the ProfileNotSupported error is raised.

  # The community candlepin container used by CI does not implement the
  # combined profile upload endpoint and does not advertise the
  # combined_reporting capability; both are provided by the subscription
  # infrastructure front-end in production. The upload itself is unit
  # tested in rhsm2, so here we verify the registration and capability
  # gating over Varlink.

  Scenario: Send() method raises error on unregistered system
    Given system is not registered
    When varlink method is called and error is expected
      | method | interface                               | arguments |
      | Send   | com.redhat.rhsm.testing.content.profile | '{}'      |
    Then varlink error is raised
      """
      com.redhat.rhsm.testing.content.profile.SystemNotRegistered
      """


  Scenario: Send() method raises error on registered system without combined profile support
    Given system is registered against candlepin server
    When varlink method is called and error is expected
      | method | interface                               | arguments |
      | Send   | com.redhat.rhsm.testing.content.profile | '{}'      |
    Then varlink error is raised
      """
      com.redhat.rhsm.testing.content.profile.ProfileNotSupported
      """


  Scenario: Send() method raises error on registered system without combined profile support with metadata
    Given system is registered against candlepin server
    When varlink method is called and error is expected
      | method | interface                               | arguments                             |
      | Send   | com.redhat.rhsm.testing.content.profile | '{"metadata": {"user_agent": "foo"}}' |
    Then varlink error is raised
      """
      com.redhat.rhsm.testing.content.profile.ProfileNotSupported
      """
