@tier1
Feature: Configure features on unconnected system
  As a system administrator
  I want to configure feature preferences before connecting to Red Hat
  So that the system connects with my desired feature set

  Background:
    Given a disconnected system

  @fast
  Scenario: Default preferences show all features enabled
    When I run `rhc configure features status`
    Then exit code is 0
    And stdout contains `content`
    And stdout contains `analytics`
    And stdout contains `remote-management`
    And stdout contains `enable`

  @fast
  Scenario: Disable remote-management preference
    When I run `rhc configure features disable remote-management`
    Then exit code is 0
    And stdout contains `During registration, 'remote-management' will not be enabled`
    When I run `rhc configure features status`
    Then exit code is 0
    And stdout contains `remote-management`
    And stdout contains `skip`

  @fast
  Scenario: Disable analytics cascades to remote-management
    When I run `rhc configure features disable analytics`
    Then exit code is 0
    And stdout contains `During registration, 'remote-management' will not be enabled (depends on 'analytics')`
    And stdout contains `During registration, 'analytics' will not be enabled`
    When I run `rhc configure features status`
    Then exit code is 0
    And stdout contains `analytics`
    And stdout contains `remote-management`
    And stdout contains `skip`

  @fast
  Scenario: Enable remote-management cascades to analytics
    Given I run `rhc configure features disable analytics`
    When I run `rhc configure features enable remote-management`
    Then exit code is 0
    And stdout contains `During registration, 'analytics' will be enabled (required by 'remote-management')`
    When I run `rhc configure features status`
    Then exit code is 0
    And stdout contains `analytics`
    And stdout contains `remote-management`
    And stdout contains `enable`

  @fast
  Scenario: Preferences file is created when preferences differ from defaults
    When I run `rhc configure features disable content`
    Then exit code is 0
    And file `/var/lib/rhc/rhc-connect-features-prefs.json` exists
    And file `/var/lib/rhc/rhc-connect-features-prefs.json` contains `false`

  @fast
  Scenario: Preferences file is absent when all preferences are at their defaults
    When I run `rhc configure features disable remote-management`
    Then file `/var/lib/rhc/rhc-connect-features-prefs.json` exists
    When I run `rhc configure features enable remote-management`
    Then exit code is 0
    And file `/var/lib/rhc/rhc-connect-features-prefs.json` does not exist

  @fast
  Scenario: Unknown feature name returns a non-zero exit code
    When I run `rhc configure features enable unknown-feature`
    Then exit code is not 0
    And stderr contains `unknown-feature`

  @fast
  Scenario: JSON output format on unconnected system
    When I run `rhc configure features status --format json`
    Then exit code is 0
    And stdout is valid JSON
    And stdout JSON field `.connected` is `false`
