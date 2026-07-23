% rhc-collector 5

# NAME

rhc-collector - execute a data collector and upload its archive

# SYNOPSIS

```
/usr/libexec/rhc/rhc-collector run COLLECTOR
```

# DESCRIPTION

**rhc-collector** is the binary responsible for executing a data collector, compressing the collected data into an archive, and uploading it to Red Hat's Ingress service. It is normally invoked by systemd service units and not run directly by users.

The binary performs the following steps for a given collector:

1. Loads the collector configuration from **/usr/lib/rhc/collectors/**_COLLECTOR_**.toml**.
2. Creates a temporary directory under **/var/tmp/rhc/**.
3. Executes the collector binary at **/usr/libexec/rhc/collectors/**_COLLECTOR_ with the **collect** subcommand, using the temporary directory as the working directory.
4. If the collector exits with a non-zero status, the temporary directory is removed and **rhc-collector** exits with an error.
5. Compresses the temporary directory into a **.tar.xz** archive.
6. Uploads the archive to the Ingress service.
7. Removes both the temporary directory and the archive.

# COMMANDS

**run** *COLLECTOR*
: Execute the full collection workflow for the specified collector (load configuration, create temporary directory, run the collector, compress, upload, clean up).

# COLLECTOR API

Each data collector is an executable shipped as part of an RPM package. Collectors must conform to the following API contract.

## Identification

Each collector has a unique ID in reverse-domain notation (e.g., **com.redhat.minimal**, **org.foreman**). The ID must match the regular expression **^[a-z0-9]+\.[a-z0-9]+(\.[a-z0-9]+)*$**.

## Executable

The collector executable must be installed at **/usr/libexec/rhc/collectors/**_COLLECTOR_. It must implement a **collect** subcommand that writes collected data into the current working directory.

The collector must not perform any network communication or upload. It only collects data into the working directory; **rhc-collector** handles compression and upload.

## Configuration file

Each collector must ship a TOML configuration file at **/usr/lib/rhc/collectors/**_COLLECTOR_**.toml** with the following structure:

```
[meta]
name = "Minimal Host Inventory Collector"
feature = "analytics"
type = "ingress"

[ingress]
user = "root"
group = "root"
content_type = "application/vnd.redhat.advisor.minimal"
```

### meta section

**meta.name** (required)
: Human-readable name of the collector.

**meta.feature** (optional)
: Reserved for future use. Only **"analytics"** value is supported.

**meta.type** (required)
: Must be set to **"ingress"** for data collectors that use the standard upload workflow.

### ingress section

**ingress.user** (optional)
: System user under which the collector runs. Defaults to **"root"**.

**ingress.group** (optional)
: System group under which the collector runs. Defaults to **"root"**.

**ingress.content_type** (required)
: MIME content type used when uploading the archive to the Ingress service (e.g., **"application/vnd.redhat.advisor.minimal"**).

## systemd units

Each collector must ship a systemd service and timer unit. Unit file names must be prefixed with **rhc-collector-** and use the collector ID as the base name:

- **rhc-collector-**_COLLECTOR_**.service**
- **rhc-collector-**_COLLECTOR_**.timer**

The service unit invokes **/usr/libexec/rhc/rhc-collector run** _COLLECTOR_.

# FILES

**/usr/libexec/rhc/rhc-collector**
: The rhc-collector binary.

**/usr/libexec/rhc/collectors/**
: Directory containing collector executables. Each file name matches the collector ID.

**/usr/lib/rhc/collectors/**
: Directory containing collector TOML configuration files.

**/var/tmp/rhc/**
: Parent directory for temporary directories created during data collection.

**/var/cache/rhc/collectors/**
: Directory containing cached execution data (start time, finish time, exit code) in JSON format.

# EXIT STATUS

**0**: Collection and upload completed successfully.

**Non-zero**: An error occurred.

# SEE ALSO

**rhc(1)**, **rhc-collector(8)**, **rhc-connect(8)**
