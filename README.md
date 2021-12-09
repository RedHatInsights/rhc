`rhc` is a simple, one-step front-end client for Red Hat connected services. It
is built to be an alternative client to `subscription-manager`,
`insights-client`, and any other client utility that enables connecting a system
to Red Hat services.

It currently performs 3 steps when it connects a system:

1. Registers the system with Red Hat Subscription Management. If the system is
   already registered, this step is a noop and it moves to the next step.
2. Registers the system with Red Hat Insights. If the system is already
   registered, this step is a noop and it moves to the next step.
3. Activate the `rhcd` daemon.

Likewise, when rhc is disconnecting a system, it performs the steps in
descending order.

1. Deactivates the `rhcd` daemon.
2. Unregisters the system from Red Hat Insights.
3. Unregisters the system from Red Hat Subscription Management.

`rhc` (the front-end client) is not the same thing as
[`yggdrasil`](https://github.com/RedHatInsights/yggdrasil). `rhc` began
as a program within the `yggdrasil` project, but has since been forked out.
`rhc` still has a soft dependency on `yggdrasil`; `yggdrasil` provides `rhcd`
(or `yggd`), the daemon that `rhc` activates as the last step in its connection
process.

## Non-goals

* `rhc` will never be a 100% compatible drop-in replacement for
  `subscription-manager`, `insights-client`, or any other Red Hat connected
  services command-line utility.
* Complexity; `rhc` is deliberately designed to be simple, and as "hands-off" as
  possible.
