# Architecture

## Exit behavior

Try to follow `sysexits.h(3)` values when returning a status to the user (64 for a bad flag, 65 for a bad value; 1 for a generic error).
Aside from separating zero and non-zero, exit code values are considered internal implementation and should not be relied on externally.

A partially failed command (i.e., `rhc connect` that manages to obtain an identity but fails to enable analytics) should return a non-zero exit code.
It will stay registered however: the operations are **not** atomic.
