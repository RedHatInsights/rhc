# Developing the rhc SELinux policy

The policy sources in this directory define the confined domains used by
`rhc-collector`. Edit these files and build a local module when changing policy;
`rhc.spec` uses the same SELinux development Makefile when packaging the
`rhc-selinux` subpackage.

## Policy changes

- `rhc.te` declares types, domains, transitions, and access rules. Add the
  narrowest rule needed for the observed operation, using the actual source
  domain, target type, object class, and permission from the AVC.
- `rhc.fc` maps installed paths to the file types declared in `rhc.te`. After
  changing a path mapping, relabel the affected paths on the test system.
- `rhc.if` contains the public interfaces other policy modules can call. Keep
  implementation details behind interfaces where appropriate. An `.if` change
  is installed at `/usr/share/selinux/devel/include/distributed/rhc.if` for
  policy development and is distinct from the loaded `.pp` module. Both files
  ship in `rhc-selinux`. Reinstall that package (or place the updated `.if` at
  that path), then rebuild any consuming policy modules that call the changed
  interface.

**Never use `unconfined_domain()` or wildcard rules.** Do not introduce
unconfined domains or rules that grant access through wildcard
types, domains, classes, or permissions (for example, `allow * *:* *;`). Do not
paste broad `audit2allow` output into `rhc.te`. File-context expressions in
`rhc.fc` are path patterns; keep them limited to the specific `rhc` paths they
label.

## Build and load a local module

Use a disposable RHEL, CentOS Stream, or Fedora test machine with SELinux
enabled. Install the SELinux policy development tools if needed:

```shell
sudo dnf install selinux-policy-devel policycoreutils policycoreutils-python-utils audit setools-console
```

From the repository root, build the policy module with the same Makefile used
by RPM packaging:

```shell
make -C selinux -f /usr/share/selinux/devel/Makefile rhc.pp
```

The generated `selinux/rhc.pp` is a local build artifact and is ignored by git.
Load it into the test machine's active policy:

```shell
sudo semodule -i selinux/rhc.pp
```

Verify that the module is installed and its types are present:

```shell
sudo semodule -l | grep rhc
```

And additionally check if the related types are present:

```shell
sudo seinfo -t | grep rhc_
```

Confirm SELinux is enforcing while verifying the change:

```shell
getenforce
```

The result should be `Enforcing`. Do not switch the machine to permissive mode
to make a policy change appear to work.

If `rhc.fc` changed, relabel only the affected installed paths after loading
the module. For example, for the paths currently covered by this module:

```shell
sudo restorecon -RFv /usr/libexec/rhc /usr/lib/rhc /var/cache/rhc
```

For a changed mapping under another path, pass that exact path to `restorecon`.
Check labels with `ls -Zd PATH` or `matchpathcon PATH`; adjust `PATH` to the
installed file or directory being checked. Avoid relabeling all of `/var/tmp`
for the policy's `rhc` temporary workspace mapping.

## Reproduce and investigate an AVC

Reproduce the denied operation on the enforcing test machine, using the same
collector or service action that exposed the problem. The bundled minimal
collector can be run with:

```shell
sudo systemctl start rhc-collector-com.redhat.minimal.service
```

Or using rhc:

```shell
sudo rhc collector enable --now com.redhat.minimal
```

That service runs the `rhc_collector_t` oneshot runner, which transitions the
bundled collector into `rhc_collector_minimal_t`. Third-party collectors
without their own policy run in the fallback `rhc_collector_plugin_t` domain.
The AVC's `scontext` identifies the domain that made the denied access; use it
for the source type in a rule rather than inferring the domain from the
systemd service. Use the action for the affected collector or domain when
investigating another path. Then inspect recent AVCs:

```shell
sudo ausearch -m AVC,USER_AVC -ts recent -i
```

Look at the complete denial, especially `scontext` (source domain), `tcontext`
(target type), `tclass` (object class), and the denied permission in braces.
Confirm that the source is the expected confined rhc domain and that the target
has the intended label (`ls -Z PATH`, `matchpathcon PATH`). A wrong label,
ordinary file ownership or mode issue, or a missing prerequisite policy module
may explain the failure without needing a new allow rule.

`audit2allow` can help explain denials or draft a candidate rule for review. For
example, save the raw events and inspect a reference-policy suggestion:

```shell
sudo ausearch -m AVC,USER_AVC -ts recent --raw > /tmp/rhc-avcs.log
audit2allow -w -i /tmp/rhc-avcs.log
audit2allow -R -i /tmp/rhc-avcs.log
```

Treat this output only as a clue. Check each event against the intended rhc
behavior and the current labels, then write a minimal, explicit rule in
`rhc.te`; do not copy broad suggestions or wildcard permissions into it.

For example, if a reviewed AVC shows a confined source domain named
`rhc_collector_t`, a specific target type named `example_state_t`, class `file`,
and only permission `getattr`, the corresponding rule shape is:

```te
allow rhc_collector_t example_state_t:file getattr;
```

Here `example_state_t` stands for the real, verified target type from the AVC;
it is not a type to add literally. Add only the permissions justified by the
operation. If the source is unconfined, the target label is unexpected, or the
suggested rule is broad, fix the cause or investigate the domain design rather
than weakening the policy.

For a denial from the bundled child collector, use its AVC `scontext` as the
source. For example, if that context is `rhc_collector_minimal_t`, the rule
shape for the same target, class, and permission is:

```te
allow rhc_collector_minimal_t example_state_t:file getattr;
```

Replace `example_state_t` with the real, verified target type. A denial from
the runner or fallback collector would instead use `rhc_collector_t` or
`rhc_collector_plugin_t`, respectively, when that is the AVC's source domain.

## Rebuild and verify one rule change

After editing `rhc.te` (or `rhc.fc`), rebuild and load the module again:

```shell
make -C selinux -f /usr/share/selinux/devel/Makefile rhc.pp
sudo semodule -i selinux/rhc.pp
```

If `rhc.fc` changed, relabel the affected path as described above. Repeat the
same operation that produced the AVC, then check for a new denial:

```shell
sudo ausearch -m AVC,USER_AVC -ts recent -i
```

Verify that the process is running in the expected confined domain (for
example, inspect it with `ps -eZ` while it is running), the target has the
intended file type, and the operation succeeds. The specific denial should no
longer recur; unrelated AVCs still need separate investigation. Review the
source diff to ensure the committed change contains only the justified rule or
path mapping, not the generated `.pp` file or `audit2allow` output.

## Package build

The RPM build compiles `selinux/rhc.te` and `selinux/rhc.fc` into `rhc.pp`,
compresses the module, and installs `rhc.if` for other policy modules. To
exercise the packaging path, follow the repository's [packaging instructions](../CONTRIBUTING.md#packaging).
