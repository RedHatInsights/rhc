# Prerequisites

Currently `rhc` is written under the assumption that it is running on a Red Hat
Enterprise Linux distribution. It will likely work on other RHEL-compatible
distributions, but may require additional packages or adjustments in order to
fully function.

# Building

It is possible to build `rhc` binary simply by running:

```
meson setup builddir
meson compile -C builddir
```

The resulting binary will be in the `builddir` directory.

If you want to install `rhc` into system, then it is recommended to create
RPM package and install rhc using package manager like dnf. You can
create RPM package using [packit](https://packit.dev/docs/cli) CLI too:

```
packit build locally
```

# Remote Debugging

If you run `rhc` in a virtual machine, it is still possible to run `rhc` in a
debugger. Within the virtual machine, install `dlv`:

```
sudo go install github.com/go-delve/delve/cmd/dlv@latest
```

You may need to open TCP port 2345 in the virtual machine. For example, to
open the port using firewalld, run:

```
sudo firewall-cmd --zone public --add-port 2345/tcp --permanent
```

Run `dlv debug`:

```
sudo /root/go/bin/dlv debug --api-version 2 --headless --listen 0.0.0.0:2345 ./ -- connect --username NNN --password ***
```

Once `dlv` is running, connect to the service, using either `dlv attach` from
your host, or by creating a launch configuration in Visual Studio Code:

```json
{
    "name": "Connect to server",
    "type": "go",
    "request": "attach",
    "mode": "remote",
    "remotePath": "${workspaceFolder}",
    "port": 2345,
    "host": "192.168.122.98"
}
```

Make sure to replace "host" with your virtual machine's IP address.

# Code Guidelines

* Commits follow the [Conventional Commits](https://www.conventionalcommits.org)
  pattern.
* Commit messages should include a concise subject line that completes the
  following phrase: "when applied, this commit will...". The body of the commit
  should further expand on this statement with additional relevant details.
* Communicate errors through return values, not logging. Library functions in
  particular should follow this guideline. You never know under which condition
  a library function will be called, so excessive logging should be avoided.
* Code should exist in a package only if it can be useful when imported
  exclusively.
* Code can exist in a package if it provides an alternative interface to
  another package, and the two packages cannot be imported together.

# Required Reading

* [Effective Go](https://go.dev/doc/effective_go)
* [CodeReviewComments](https://github.com/golang/go/wiki/CodeReviewComments)
* [Go Proverbs](https://go-proverbs.github.io/)

In addition to these 3 "classics", [A collection of Go style
guides](https://golangexample.com/a-collection-of-go-style-guides/) contains a
wealth of resources on writing idiomatic Go.

# Contact

Chat on Matrix: [#yggd:matrix.org](https://matrix.to/#/#yggd:matrix.org).
