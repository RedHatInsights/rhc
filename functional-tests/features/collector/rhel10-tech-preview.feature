@tier2
Feature: rhc collector (RHEL 10 tech preview)

  Scenario: New system registration
    Given a connected system
     Then the system is connected
      And the system has content
      And all data collectors are disabled

  @wip
  Scenario: User opting into the tech preview
    Given a connected system
     When I run `insights-client --unregister`
      And I run `rhc collector enable com.redhat.minimal`
     Then systemd unit `rhc-collector-com.redhat.minimal.timer` is enabled

  @wip
  Scenario: Conservative user opting into the tech preview
    Given a system registered with subscription-manager
     When I run `rhc collector enable com.redhat.minimal`
     Then systemd unit `rhc-collector-com.redhat.minimal.timer` is enabled

  @wip
  Scenario: Auditing and compliance
    Given a connected system
     When I start systemd unit `rhc-collector-com.redhat.minimal.service`
     Then journal for unit `rhc-collector-com.redhat.minimal.service` contains `com.redhat.minimal`
      And journal for unit `rhc-collector-com.redhat.minimal.service` contains `return code`
      And journal for unit `rhc-collector-com.redhat.minimal.service` contains `User-Agent`
      And journal for unit `rhc-collector-com.redhat.minimal.service` contains `X-Rh-Insights-Request-Id`

  @wip
  Scenario: Transparency
     When I run `/usr/libexec/rhc/collector/com.redhat.minimal collect` in a temporary directory
     Then the temporary directory is not empty
