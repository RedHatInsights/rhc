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
    When varlink method is called and error is expected
      | method  | interface                       | arguments          |
      | Refresh | com.redhat.rhsm.testing.content | '{"force": false}' |
    Then varlink error is raised
      """
      com.redhat.rhsm.testing.content.CertificateUpToDate
      """


  Scenario: Refresh() method regenerates deleted redhat.repo file, when certificate is up to date
    Given system is registered against candlepin server
    Given entitlement certificate is installed
    And redhat.repo file is deleted
    When varlink method is called and error is expected
      | method  | interface                       | arguments          |
      | Refresh | com.redhat.rhsm.testing.content | '{"force": false}' |
    Then varlink error is raised
      """
      com.redhat.rhsm.testing.content.CertificateUpToDate
      """
    And redhat.repo file exists
    And entitlement certificate has still the same name and i-node


  @fixture.redhat_repo_file_not_writable
  Scenario: Refresh() method raises IOFailed error, when redhat.repo cannot be written and certificate is up to date
    Given system is registered against candlepin server
    Given entitlement certificate is installed
    And redhat.repo file does not exist
    When varlink method is called and error is expected
      | method  | interface                       | arguments          |
      | Refresh | com.redhat.rhsm.testing.content | '{"force": false}' |
    Then varlink error is raised
      """
      com.redhat.rhsm.testing.content.IOFailed
      """
    And entitlement certificate has still the same name and i-node


  @fixture.redhat_repo_file_not_writable
  Scenario: Refresh() method raises IOFailed error, when redhat.repo cannot be written and certificate is force updated
    Given system is registered against candlepin server
    Given entitlement certificate is installed
    And redhat.repo file does not exist
    When varlink method is called and error is expected
      | method  | interface                       | arguments         |
      | Refresh | com.redhat.rhsm.testing.content | '{"force": true}' |
    Then varlink error is raised
      """
      com.redhat.rhsm.testing.content.IOFailed
      """


# Skip this scenario, because this issue has to be fixed in rhsm2
# https://github.com/jirihnidek/rhsm2/issues/117
#  Scenario: Refresh() method succeeds on registered system and entitlement certificate is missing
#    Given system is registered against candlepin server
#    # This is for the case, when something or somebody has deleted entitlement certificate or key
#    # The Refresh() method has to be able to regenerate given certificate and key
#    And no entitlement certificate and key is installed
#    When varlink method is called
#      | method  | interface                       | arguments          |
#      | Refresh | com.redhat.rhsm.testing.content | '{"force": false}' |
#    Then method call was successful
#    And method returned JSON compliant with 'com.redhat.rhsm.testing.content.Refresh.json' schema


  Scenario: Refresh() method succeeds on registered system after regenerating of entitlement certificate
    Given system is registered against candlepin server
    Given entitlement certificate is installed
    Given entitlement certificate is regenerated on candlepin server
    # We have to wait one second here, because If-Modified-Since HTTP header can be sent only in
    # time format defined in RFC 822, which has only second precision. If Refresh() method was
    # called immediately, then it would lead to 304 HTTP code (not modified), because the time
    # of creating old and new certificate with second precision would be the same.
    Then wait '1' seconds
    When varlink method is called
      | method  | interface                       | arguments          |
      | Refresh | com.redhat.rhsm.testing.content | '{"force": false}' |
    Then method call was successful
    And method returned JSON compliant with 'com.redhat.rhsm.testing.content.Refresh.json' schema
    And entitlement certificate has different name and i-node
    And installed entitlement certificate contains 'BEGIN CERTIFICATE'
    And installed entitlement certificate contains 'BEGIN ENTITLEMENT DATA'
    And installed entitlement certificate contains 'BEGIN SIGNATURE'


  Scenario: Refresh() method succeeds on registered system with metadata
    Given system is registered against candlepin server
    Given entitlement certificate is installed
    When varlink method is called and error is expected
      | method  | interface                       | arguments                                             |
      | Refresh | com.redhat.rhsm.testing.content | '{"force": false, "metadata": {"user_agent": "foo"}}' |
    Then varlink error is raised
      """
      com.redhat.rhsm.testing.content.CertificateUpToDate
      """
    And entitlement certificate has still the same name and i-node


  Scenario: Refresh(force=true) method succeeds on registered system
    Given system is registered against candlepin server
    Given entitlement certificate is installed
    When varlink method is called
      | method  | interface                       | arguments         |
      | Refresh | com.redhat.rhsm.testing.content | '{"force": true}' |
    Then method call was successful
    And method returned JSON compliant with 'com.redhat.rhsm.testing.content.Refresh.json' schema
    And entitlement certificate has still the same name and i-node
    And installed entitlement certificate contains 'BEGIN CERTIFICATE'
    And installed entitlement certificate contains 'BEGIN ENTITLEMENT DATA'
    And installed entitlement certificate contains 'BEGIN SIGNATURE'


  Scenario: Refresh(force=true) method succeeds on registered system after regenerating of entitlement certificate
    Given system is registered against candlepin server
    Given entitlement certificate is installed
    Given entitlement certificate is regenerated on candlepin server
    # We have to wait one second here, because If-Modified-Since HTTP header can be sent only in
    # time format defined in RFC 822, which has only second precision. If Refresh() method was
    # called immediately, then it would lead to 304 HTTP code (not modified), because the time
    # of creating old and new certificate with second precision would be the same.
    Then wait '1' seconds
    When varlink method is called
      | method  | interface                       | arguments          |
      | Refresh | com.redhat.rhsm.testing.content | '{"force": true}' |
    Then method call was successful
    And method returned JSON compliant with 'com.redhat.rhsm.testing.content.Refresh.json' schema
    And entitlement certificate has different name and i-node
    And installed entitlement certificate contains 'BEGIN CERTIFICATE'
    And installed entitlement certificate contains 'BEGIN ENTITLEMENT DATA'
    And installed entitlement certificate contains 'BEGIN SIGNATURE'


  Scenario: Refresh() called twice on a registered system succeeds both times
    Given system is registered against candlepin server
    Given entitlement certificate is installed
    When varlink method is called and error is expected
      | method  | interface                       | arguments          |
      | Refresh | com.redhat.rhsm.testing.content | '{"force": false}' |
    Then varlink error is raised
      """
      com.redhat.rhsm.testing.content.CertificateUpToDate
      """
    And entitlement certificate has still the same name and i-node
    And varlink method is called and error is expected
      | method  | interface                       | arguments          |
      | Refresh | com.redhat.rhsm.testing.content | '{"force": false}' |
    Then varlink error is raised
      """
      com.redhat.rhsm.testing.content.CertificateUpToDate
      """
    And entitlement certificate has still the same name and i-node
