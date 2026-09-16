% com.redhat.rhc 7

# NAME

com.redhat.rhc - Varlink API provided by rhc-server

# SYNOPSIS

```
varlinkctl call unix:/run/rhc/com.redhat.rhc INTERFACE.METHOD '{ARGUMENTS}'
```

# DESCRIPTION

**rhc-server** exposes a set of Varlink interfaces on a Unix domain socket at **/run/rhc/com.redhat.rhc**. These interfaces allow local programs to query data collectors without requiring direct access to the **rhc** CLI.

The service is managed by systemd. The socket unit **rhc-server.socket** supports socket activation: first Varlink call to the socket starts **rhc-server.service** automatically if it is not already running.

## Connecting to the service

All methods are called over the Unix domain socket using the **varlinkctl**(1) tool or any Varlink-compatible client. The socket address is:

```
unix:/run/rhc/com.redhat.rhc
```

To introspect the full list of interfaces provided by the running service:

```
varlinkctl introspect unix:/run/rhc/com.redhat.rhc
```

To introspect a specific interface:

```
varlinkctl introspect unix:/run/rhc/com.redhat.rhc com.redhat.rhc.collector
```

# INTERFACES

The following Varlink interfaces are served on the socket:

- **com.redhat.rhc.collector** - Query registered data collectors.

# INTERFACE com.redhat.rhc.collector

Query data collectors registered with **rhc-server**. This interface does not require the system to be registered.

## Types

**CollectorInfo**

Detailed information about a single collector.

```
type CollectorInfo (
    id: string,
    name: string,
    feature: ?string,
    last_run: ?int,
    next_run: ?int,
    config_path: string,
    service_name: string,
    timer_name: string
)
```

- **id** - Unique collector identifier in reverse-domain notation (e.g. _com.redhat.minimal_).
- **name** - Human-readable name of the collector.
- **feature** - The feature gate this collector belongs to (e.g. _analytics_), or null if none.
- **last_run** - Unix timestamp of the last completed run, or null if the collector has not run yet.
- **next_run** - Unix timestamp of the next scheduled run from the systemd timer, or null if not scheduled.
- **config_path** - Absolute path to the collector TOML configuration file.
- **service_name** - Name of the systemd service unit for this collector.
- **timer_name** - Name of the systemd timer unit for this collector.

## Methods

**List() -> (collectors: []CollectorInfo)**

Return all registered collectors with full details.

```
$ varlinkctl call unix:/run/rhc/com.redhat.rhc \
    com.redhat.rhc.collector.List '{}'
```

Example response:

```json
{
    "collectors" : [
        {
            "config_path" : "/usr/lib/rhc/collectors/com.redhat.minimal.toml",
            "feature" : "analytics",
            "id" : "com.redhat.minimal",
            "name" : "Minimal Host Inventory Collector",
            "service_name" : "rhc-collector-com.redhat.minimal.service",
            "timer_name" : "rhc-collector-com.redhat.minimal.timer"
        }
    ]
}
```

**Info(id: string) -> (info: CollectorInfo)**

Return detailed information about a single collector identified by **id**.

```
$ varlinkctl call unix:/run/rhc/com.redhat.rhc \
    com.redhat.rhc.collector.Info '{"id": "com.redhat.minimal"}'
```

Example response:

```json
{
    "info" : {
        "config_path" : "/usr/lib/rhc/collectors/com.redhat.minimal.toml",
        "feature" : "analytics",
        "id" : "com.redhat.minimal",
        "name" : "Minimal Host Inventory Collector",
        "service_name" : "rhc-collector-com.redhat.minimal.service",
        "timer_name" : "rhc-collector-com.redhat.minimal.timer"
    }
}
```

## Errors

**InvalidParameter(parameter: string)**
: Returned by **Info** when a parameter value is invalid (e.g. malformed collector ID).

**NoSuchCollector(id: string)**
: Returned by **Info** when no collector with the given **id** exists.

# SOCKET ACTIVATION

The **rhc-server.socket** systemd unit listens on **/run/rhc/com.redhat.rhc** and starts **rhc-server.service** on demand when a client connects. To enable socket activation:

```
# systemctl enable --now rhc-server.socket
```

To verify the socket is listening:

```
$ systemctl status rhc-server.socket
```

To check if the service is running:

```
$ systemctl status rhc-server.service
```

If the socket is disabled, **rhc** CLI commands that depend on the service will fail with an error suggesting to restart the socket:

```
# systemctl restart rhc-server.socket
```

# FILES

**/run/rhc/com.redhat.rhc**
: Unix domain socket for the Varlink service, owned and only connectable by root (0600).

**/run/rhc/rhc-server.pid**
: PID file ensuring only one instance of **rhc-server** runs at a time.

**/usr/libexec/rhc/rhc-server**
: The **rhc-server** binary.

**/usr/lib/systemd/system/rhc-server.service**
: Systemd service unit for **rhc-server**.

**/usr/lib/systemd/system/rhc-server.socket**
: Systemd socket unit for Varlink socket activation.

# SEE ALSO

**rhc(1)**, **rhc-collector(5)**, **rhc-collector(8)**, **varlinkctl(1)**, **systemctl(1)**
