# Architecture

rhc codebase is separated into three hierarchical layers: `cmd/` → `pkg/` → `internal/`.
You may treat it as a mix of layered and ports-and-adapters architecture: `cmd/` is an input adapter (with `*Options` structs being the ports), `pkg/` is the application core, and `internal/` output adapters that wrap system interfaces, filesystem, or host binaries.

- **Presentation** contains input adapters in `cmd/`: it takes input from the user and presents them back the output. It translates presentation-specific input (CLI flags, Varlink objects) into an internal representation the business layer understands. Raw input is captured into an `Input` object, validated, and passed forward as an `Options`. Presentation code must not import from `internal/`, it should always go through `pkg/`.
- **Business** contains the application core in `pkg/`. It owns operations, functions that map to product use-cases such as `connect` or `configure features`. This is where shared types and constants live.
- **Internal** contains output adapters in `internal/`. It interfaces with the host system by interacting with system tools or external APIs over HTTP or D-Bus. Information should generally be passed in by the business layer rather than pulled.

It is important to note the only public API is the command line interface and the Varlink objects and methods.
rhc is not meant to be a library, and the major version will _not_ be bumped when a breaking change is made to methods or structs.

## Exit behavior

Try to follow `sysexits.h(3)` values when returning a status to the user (64 for a bad flag, 65 for a bad value; 1 for a generic error).
Aside from separating zero and non-zero, exit code values are considered internal implementation and should not be relied on externally.

A partially failed command (i.e., `rhc connect` that manages to obtain an identity but fails to enable analytics) should return a non-zero exit code.
It will stay registered, however: the operations are **not** atomic.
