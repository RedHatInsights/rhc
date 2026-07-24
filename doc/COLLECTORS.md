# Collectors

Collectors gather data from the host, compress it to an archive and upload it to Red Hat Hybrid Cloud Console. The rhc executable is used to show the timer schedule and the status of each collector. A collector constitutes a distinct executable, a configuration file and systemd units.

## Prerequisites

Collectors and the `rhc collector` commands assume the following dependencies:

**Platform:** Red Hat Enterprise Linux, CentOS Stream, or Fedora

**rhc package:** Provides `rhc-collector`, the collector config directory (`/usr/lib/rhc/collectors/`), and the collector executable directory (`/usr/libexec/rhc/collectors/`)

**rhc-server:** `rhc-server.socket` must be running for `rhc collector list`, `info`, `timers` (Varlink API on `/run/rhc/com.redhat.rhc`)

**Registration:** Artifact upload requires a system registered with RHSM; consumer certificate at `/etc/pki/consumer/cert.pem` and key at `/etc/pki/consumer/key.pem`

**systemd:** Required for scheduled collector runs and for `rhc collector enable` and `disable`, which manage timer units directly

## Naming Conventions and Artifact Paths

The collector `ID` is a key used for the config filename, executable name, systemd units, timer cache, and CLI arguments.

**Format:** reverse-DNS, lowercase, dot-separated segments. A regex match on `^[a-z0-9]+\.[a-z0-9]+(\.[a-z0-9]+)*$` is used to validate the format.

**Naming convention:** use a vendor prefix (`com.redhat.*`, `com.example.*`) and a descriptive suffix (`minimal`, `advisor`, `compliance`).

**Systemd units:** unit names use the prefix `rhc-collector-{ID}`.

### Valid Collector IDs

- `com.redhat.minimal`
- `com.redhat.advisor`
- `org.example.collector.v1`
- `v1.example2.collector3`

### Invalid Collector IDs

- Empty string, single segment (`org`), or only dots (`...`)
- Uppercase (`Com.RedHat.Advisor`)
- Underscores (`com.red_hat.advisor`, `test_collector`)
- Hyphens (`com.red-hat.advisor`)
- Special characters or spaces (`com.red@hat.advisor`, `com.red hat.advisor`)
- Empty segments (`single.`, `.invalid.id`, `test..collector`, trailing dot)
- Path components (`/absolute/path/com.redhat.advisor`, `relativepath/com.redhat.id`)

### Collector Artifact Paths

Collectors are discovered from fixed paths on the host. Product collectors ship the following artifacts from their packages:

- **Config:** `/usr/lib/rhc/collectors/{ID}.toml`
- **Executable:** `/usr/libexec/rhc/collectors/{ID}`
- **Orchestrator:** `/usr/libexec/rhc/rhc-collector`
- **Service:** `/usr/lib/systemd/system/rhc-collector-{ID}.service`
- **Timer:** `/usr/lib/systemd/system/rhc-collector-{ID}.timer`
- **Timer cache:** `/var/cache/rhc/collectors/{ID}.json` (generated at runtime)

The collector `ID` must match exactly across every shipped artifact. For collector ID `com.redhat.example`, the filenames and unit names are:

- **Config:** `/usr/lib/rhc/collectors/com.redhat.example.toml`
- **Executable:** `/usr/libexec/rhc/collectors/com.redhat.example`
- **Service:** `/usr/lib/systemd/system/rhc-collector-com.redhat.example.service`
- **Timer:** `/usr/lib/systemd/system/rhc-collector-com.redhat.example.timer`
- **Timer cache:** `/var/cache/rhc/collectors/com.redhat.example.json` (generated at runtime)

`rhc collector list` discovers collectors from valid TOML config files in `/usr/lib/rhc/collectors/`. It does not verify that the executable exists. A missing or non-executable file fails later when `rhc-collector` runs the collection.

For the in-tree reference collector implementation `com.redhat.minimal`, files exist under `cmd/minimal-collector/` for the executable, `data/collectors/com.redhat.minimal.toml` for the TOML config, and `data/systemd/rhc-collector-com.redhat.minimal.{service,timer}` for the service and timer units. Those paths are packaged by `rhc.spec` into the install locations listed above.

## Collector Config Schema

```toml
[meta]
name = "Example collector name"                             # required
type = "ingress"                                            # required; "ingress" is supported
feature = "analytics"                                       # optional; "analytics" is supported

[ingress]
user = "root"                                               # optional; "root" as default
group = "root"                                              # optional; "root" as default
content_type = "application/vnd.redhat.example.collection"  # required
```

Note: `ingress.content_type` must be coordinated with the Ingress/backend owners as it identifies the payload type on upload.

## Collector Executable

A collector can be any executable (Go, Python, Shell etc). The executable must fulfill each of the following requirements:

1. Accept exactly one subcommand: `collect`
2. Write output into the current working directory (set by `rhc-collector` to a temp dir under `/var/tmp/rhc/`)
3. Exit 0 on success, non-zero on failure
4. Take no untrusted user input. The collector executable runs as the user and group configured in the collector TOML (default: root).

## Collector Archive Format

The `rhc-collector` archives what the collector writes to its working directory in a tar.xz format. The expected layout of the archive is defined by the Ingress service and backend that consume it. For further information consult the references below.

References:
- [insights-ingress-go](https://github.com/RedHatInsights/insights-ingress-go) — Ingress upload service
- Host-based inventory (Red Hat internal): https://inscope.corp.redhat.com/docs/default/component/host-based-inventory/#host-deduplication
- In-tree example for one payload type: `cmd/minimal-collector/`

## Systemd Service Unit

Service file name: `rhc-collector-com.redhat.example.service`

```ini
[Unit]
Description=Example data collector
After=network.target
Documentation=https://github.com/RedHatInsights/rhc

# Only try to restart six times
StartLimitIntervalSec=12h
StartLimitBurst=6

[Service]
Type=oneshot
ExecStart=/usr/libexec/rhc/rhc-collector run com.redhat.example

Restart=on-failure
RestartSec=1h

[Install]
WantedBy=multi-user.target
```

## Systemd Timer Unit

Timer file name: `rhc-collector-com.redhat.example.timer`

```ini
[Unit]
Description=Example collector timer
Documentation=https://github.com/RedHatInsights/rhc

[Timer]
OnCalendar=daily
RandomizedDelaySec=4h

# Run if the system was down
Persistent=true

[Install]
WantedBy=timers.target
```

## Shipping a Collector (External Packages)

Do not add new product collectors to the rhc repository. Collectors should be shipped from a separate owning package and install the artifacts described below.

The examples below use collector ID `com.redhat.example`.

### 1. Choose ID and Content Type

- Pick a valid reverse-DNS ID (for example, `com.redhat.example`).
- Request or confirm an `application/vnd.redhat.*` content type with the Ingress/backend owners.

### 2. Implement the Collector Executable

Build an executable named `{ID}` that implements a `collect` subcommand and writes output to the current working directory. See [Collector Executable](#collector-executable) for more detail.

### 3. Configuration

Package a config TOML file using the [Collector Config Schema](#collector-config-schema) and install it to `/usr/lib/rhc/collectors/{ID}.toml`.

### 4. Systemd Units

Package service and timer units based on
[Systemd Service Unit](#systemd-service-unit) and
[Systemd Timer Unit](#systemd-timer-unit), and install them to `/usr/lib/systemd/system/`.

Include the following in the unit files:
- `Description` in both units
- `ExecStart` in the service unit to pass your collector ID to `rhc-collector run`
- `OnCalendar` and `RandomizedDelaySec` in the timer as needed.

The timer does not reference the collector executable directly. Systemd starts the paired service by name (`rhc-collector-{ID}.timer` activates `rhc-collector-{ID}.service`), the service invokes `rhc-collector run {ID}`, which in turn runs `/usr/libexec/rhc/collectors/{ID} collect`.

Use the `com.redhat.minimal` entries in `rhc.spec` and `data/systemd/` as a reference for install paths and `%systemd_post` / `%systemd_preun` / `%systemd_postun` packaging macros.

### 5. Verify a Collector Locally

Prerequisites: `rhc` installed. Run the following as root:

1. Install the collector files:

```shell
COLLECTOR_ID=com.redhat.example

# Install artifacts (adjust source paths to your build output)
install -D -m 0644 "${COLLECTOR_ID}.toml" "/usr/lib/rhc/collectors/${COLLECTOR_ID}.toml"
install -D -m 0755 "${COLLECTOR_ID}" "/usr/libexec/rhc/collectors/${COLLECTOR_ID}"
install -D -m 0644 "rhc-collector-${COLLECTOR_ID}.service" "/usr/lib/systemd/system/rhc-collector-${COLLECTOR_ID}.service"
install -D -m 0644 "rhc-collector-${COLLECTOR_ID}.timer" "/usr/lib/systemd/system/rhc-collector-${COLLECTOR_ID}.timer"
systemctl daemon-reload
```

2. Confirm `rhc-server` is available:

```shell
systemctl is-active rhc-server.socket
```

3. Test the collector executable in isolation (no upload):

```shell
mkdir -p /tmp/collector-test && cd /tmp/collector-test
/usr/libexec/rhc/collectors/"${COLLECTOR_ID}" collect
ls -R
```

4. Verify discovery via `rhc`:

```shell
rhc collector list
rhc collector info "${COLLECTOR_ID}"
rhc collector timers
```

5. Run the full collection pipeline (collect, archive, upload):

```shell
rhc-collector run "${COLLECTOR_ID}"
```

6. Enable the timer and trigger an immediate run:

```shell
rhc collector enable "${COLLECTOR_ID}" --now
```

7. Check timer status and last-run cache:

```shell
systemctl status "rhc-collector-${COLLECTOR_ID}.timer"
cat "/var/cache/rhc/collectors/${COLLECTOR_ID}.json"
```

## See Also

[CONTRIBUTING.md](../CONTRIBUTING.md) - general build, packaging, and code guidelines
