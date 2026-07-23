% rhc-collector 8

# NAME

rhc-collector - Manage data collectors for Red Hat hosted services

# SYNOPSIS

```
rhc collector list [--format json]
rhc collector timers [--format json]
rhc collector info [--format json] COLLECTOR
rhc collector enable [--now] COLLECTOR
rhc collector disable [--now] COLLECTOR
```

# DESCRIPTION

The **rhc collector** command manages data collectors that gather system data and upload it to Red Hat hosted services. Each collector is identified by a reverse-domain-notation ID (e.g., **com.redhat.minimal**) and runs on a systemd timer.

Collectors are shipped by Red Hat teams as RPM packages. Each package provides an executable, a TOML configuration file, and systemd service and timer units. The **rhc collector** command provides a unified interface to list, inspect, enable, and disable these collectors.

Data collection itself is performed by the **/usr/libexec/rhc/rhc-collector** binary, which is invoked by the systemd service units. See **rhc-collector(5)** for details on the binary and the collector API.

# SUBCOMMANDS

**list** [**--format** json]
: List all available collectors. Displays the collector ID and human-readable name. When **--format json** is specified, output is printed as a JSON array.

**timers** [**--format** json]
: List timer information for all available collectors. Displays the collector ID, time since the last run finished, and time until the next scheduled run. When **--format json** is specified, output is printed as a JSON array.

**info** [**--format** json] *COLLECTOR*
: Display detailed information about a specific collector, including its name, feature, last and next run times, configuration file path, and associated systemd units. When **--format json** is specified, output is printed as a JSON object.

**enable** [**--now**] *COLLECTOR*
: Enable the systemd timer for the specified collector. The collector will run on its configured schedule. If **--now** is specified, an immediate collection is also triggered.

**disable** [**--now**] *COLLECTOR*
: Disable the systemd timer for the specified collector. If the collector is currently running, it will be allowed to finish. If **--now** is specified, any running collection is stopped immediately.

# OPTIONS

**--format** json
: Print output in machine-readable JSON format. Available for the **list**, **timers**, and **info** subcommands.

**--now**
: When used with **enable**, trigger an immediate collection in addition to enabling the timer. When used with **disable**, stop any running collection immediately in addition to disabling the timer.

# EXAMPLES

**List all available collectors:**

```
# rhc collector list
ID                      NAME
com.redhat.minimal      Minimal Host Inventory Collector
com.redhat.compliance   Red Hat Lightspeed Compliance
org.foreman             Red Hat Satellite
```

**Show timer status for all collectors:**

```
# rhc collector timers
ID                      LAST    NEXT
com.redhat.minimal       18h    6h
com.redhat.compliance  3h 47m   13m
org.foreman              28d    -

Hint: Run 'rhc collector info COLLECTOR' to show more details.
```

**Show detailed information about a collector:**

```
# rhc collector info com.redhat.minimal
Name:      Minimal Host Inventory Collector
Feature:   analytics

Last run:  Wed 2026-06-17 17:21 CEST (1h 15m ago)
Next run:  Wed 2026-06-17 20:00 CEST (1h 24m)

Config:   /usr/lib/rhc/collectors/com.redhat.minimal.toml
Service:  rhc-collector-com.redhat.minimal.service
Timer:    rhc-collector-com.redhat.minimal.timer
```

**Enable a collector with immediate data collection:**

```
# rhc collector enable --now com.redhat.minimal
Enabled timer rhc-collector-com.redhat.minimal.timer and triggered immediate collection.
```

**Disable a collector:**

```
# rhc collector disable com.redhat.minimal
Disabled timer rhc-collector-com.redhat.minimal.timer.
```

# SYSTEMD REQUIREMENTS

The **enable** and **disable** subcommands require systemd. In environments without systemd (e.g., containers), these commands will print an error:

```
automatic execution requires systemd, which is not present in your environment
```

In such environments, the data collection must be triggered manually. See **rhc-collector(5)** for details.

# EXIT STATUS

**0**: Success

**Non-zero**: An error occurred. Common errors include:

- Collector ID not found or invalid
- rhc-server socket not available
- systemd not present (for enable/disable)
- Timer unit not installed

# SEE ALSO

**rhc(1)**, **rhc-collector(5)**, **rhc-connect(8)**, **rhc-configure(8)**
