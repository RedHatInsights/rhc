# Security Policy

## Supported Versions

This repository contains upstream sources of the 'rhc' tool, available in Red
Hat Enterprise Linux, and select Fedora-based distributions.

A single release branch on GitHub may be targeting one or more RHEL versions.
To learn more about Red Hat Enterprise Linux Life Cycle, visit
[the Customer Portal](https://access.redhat.com/support/policy/updates/errata).

## Reporting a Vulnerability

Visit [Product Security Center](https://access.redhat.com/security/) and
[contact our security team](https://access.redhat.com/security/team/contact/)
if you believe you have discovered security-related problems.

If you are a Red Hat customer, you may also
[contact our support](https://access.redhat.com/support/).

## Security and Threat model

### Exposure surfaces

Binaries rhc consists of:

- `rhc` shell binary. Code execution requires root privileges, the user is fully trusted.
- `rhc-server` daemon and its `com.redhat.rhc` Unix socket. It is owned by root with permissions `0600`. Connections require root privileges, the caller is fully trusted.
- `rhc-collector` shell binary. Is not meant to be used by users under normal operation. It is invoked by root-owned systemd units of individual data collectors, supplying `COLLECTOR-ID` as a positional argument to its `run` command.
- `com.redhat.minimal` data collector. Code execution requires root privileges, takes no input.

APIs rhc connects to:

- `com.redhat.RHSM1` D-Bus API on a system bus. Its policy requires the caller to be `root` by default, except for read-only methods and objects.
- `org.freedesktop.systemd1` D-Bus API on a system bus.
- `cert.console.redhat.com` HTTP API. System CA certificates are used for mTLS transport.

Additionally, there are planned trust boundaries that are not yet implemented:

- `rhc.conf` configuration file and files in its drop-in directory. They are owned by root with permissions `0640` (files) and `0750` (the directory).
- `subscription.rhsm.redhat.com` HTTP API. Red Hat's CA certificates provided by subscription-manager-rhsm-certificates package are used for mTLS transport.
- Satellite HTTP APIs. When reconfigured, rhc may communicate with remote HTTP APIs served by Red Hat Satellite Server or Red Hat Satellite Capsule Server. Satellite (Katello) CA certificates are used for mTLS transport.

### Adversary model

- Local unprivileged user or process: anything that might be running on a RHEL host.
- Network adversary between the host and remote HTTP APIs.
- Compromised data collector that would get executed with root permissions by `rhc-collector`.

Out of scope:

- Attacker who already holds root permissions and can modify arbitrary files.
