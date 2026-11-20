Integrations Tests
==================

This repository contains integration test of `rhc`. These tests are located in directory
`./integrations-tests`. It is possible to run these tests locally (in some testing VM) or
it is possible to run tests using tmt. It is recommended to run integration tests in
VM, because it is required to use root user for running test. Why is root user required?
Integration tests try to connect and disconnect the system and only `root` user is
allowed to do it. Integration tests can in theory do anything (e.g. restart testing system).
Thus, it is better to use VM.

If you want to run tests locally in some testing VM (preferred variant for development of
integration tests), then it necessary to set up testing environment first.

Requirements
------------

You will have to install required RPMs and Python packages:

```
dnf -y install python3-pip python3-pytest tmt+all
sudo pip install -r integration-tests/requirements.txt
```

Configuration of Testing Environment
------------------------------------

`pytest-client-tools` reads its configuration from a `settings.toml` file that
is **not** part of this repository. CI downloads a sanitized copy (non-secret
values only) from the URL in the `SETTINGS_URL` environment variable (see
`.testing-farm.yaml`); `systemtest/tests/integration/test.sh` stores it next to
the tests before running them. For local runs you can create the file yourself
— it is git-ignored — but it must not contain real secrets.

Secret values are provided through environment variables understood by
`pytest-client-tools`:

| Setting | Environment variable |
| ------- | -------------------- |
| `candlepin.username` | `PYTEST_CLIENT_TOOLS_CANDLEPIN__USERNAME` |
| `candlepin.password` | `PYTEST_CLIENT_TOOLS_CANDLEPIN__PASSWORD` |
| `candlepin.activation_keys` | `PYTEST_CLIENT_TOOLS_CANDLEPIN__ACTIVATION_KEYS` |
| `auth_proxy.username` | `PYTEST_CLIENT_TOOLS_AUTH_PROXY__USERNAME` |
| `auth_proxy.password` | `PYTEST_CLIENT_TOOLS_AUTH_PROXY__PASSWORD` |
| `service_account.client_secret` | `PYTEST_CLIENT_TOOLS_SERVICE_ACCOUNT__CLIENT_SECRET` |

The naming rule is `PYTEST_CLIENT_TOOLS_<SECTION>__<KEY>` for nested settings
(each `.` in the dotted path becomes `__`, uppercased) and
`PYTEST_CLIENT_TOOLS_<KEY>` for top-level settings. Values are parsed as TOML:
`8443` becomes an integer, `true` a boolean, and `["key1", "key2"]` a list.
Quoting a value (e.g. `'"12345"'`) forces a string on dynaconf >= 3.3
(verified on 3.3.5); on 3.2.x and 3.1.x the value stays numeric. The
requirements floor is 3.2.13 for python >= 3.8 — a floor, not a pin:
the RHEL 10 run installs 3.3.5.
The portable rule: pass values with the intended type as TOML in
`settings.toml` / `.secrets.toml`, and do not rely on quoting an
environment variable for type.

`ENV_FOR_DYNACONF` selects which section of `settings.toml` is used
(`default`, `stage`, `prod`, `satellite*`, `testing`, `local`, ...). Sections
inherit keys they omit from the `default` section, so environment variables
only need to carry the secrets of the selected environment.

The precedence is: environment variables > `.secrets.toml` (git-ignored) >
`settings.toml`. Because the sanitized settings file carries
`candlepin.username` without the password, a password must always be
supplied through the environment when running against a real candlepin —
`pytest-client-tools` requires `candlepin.password` whenever
`candlepin.username` is set, and activation keys alone do not satisfy
that check. For example, to run the tests against stage:

```
export ENV_FOR_DYNACONF=stage
export PYTEST_CLIENT_TOOLS_CANDLEPIN__USERNAME=...
export PYTEST_CLIENT_TOOLS_CANDLEPIN__PASSWORD=...
export PYTEST_CLIENT_TOOLS_CANDLEPIN__ACTIVATION_KEYS='["..."]'
```

For local overrides (including whole custom sections) use a git-ignored
`.secrets.toml` file.

The `[testing]` section targets the containerized Candlepin used by manual
containerized runs; no automated job sets `ENV_FOR_DYNACONF=testing` today
(CI jobs pick their environment at Testing Farm request time). Its
container credentials are not part of the sanitized settings file either;
`systemtest/tests/integration/test.sh` exports the well-known container
values when `ENV_FOR_DYNACONF=testing` and no explicit credentials are set.

The encrypted blobs in `.testing-farm.yaml` are rotated by the manual
`rotate-tf-secrets.sh` script and a reviewed PR (see the CCT credentials
how-to on the QE wiki); there is no automated sync job for them.

### Vault-delivered credentials

Testing Farm delivers four secrets to credentialed runs of this
repository (`.testing-farm.yaml`): `SETTINGS_URL`, `VAULT_ADDR`,
`VAULT_ROLE_ID` and `VAULT_SECRET_ID`. The served settings file carries
non-secret values directly and secret values as `@vault:` sentinels: at
config load, pytest-client-tools authenticates to the Vault instance
named by `VAULT_ADDR` with the AppRole pair (`VAULT_ROLE_ID` +
`VAULT_SECRET_ID`) and reads each sentinel's value from the
`apps/data/cct-ci/*` KV namespace. Tokens issued by the AppRole carry
the `app-ro-cct-ci` policy — read-only on that namespace, writes denied
— and are short-lived (20 min TTL, 10 h max). `PYTEST_CLIENT_TOOLS_*`
environment variables (or a local `.secrets.toml`) override any
resolved value.

Fork protection is structural, not policy: Testing Farm generates an
RSA key pair per git repository URL and per API token, so a ciphertext
decrypts only for a request against this repository's URL submitted
with the token that encrypted it. A fork's CI runs against a different
repository URL and cannot decrypt the blobs.

Strict mode: `systemtest/tests/integration/test.sh` exports
`PYTEST_CLIENT_TOOLS_VAULT_REQUIRED=1` whenever it downloads the served
settings file, so an `@vault:` sentinel that cannot be resolved fails
loudly at config load instead of degrading into a missing key and a
skipped run. Runs without `SETTINGS_URL` (containerized, manual) are
unaffected.

The two public-ranch Packit jobs (`integration-tests/centos-stream` and
`integration-tests/fedora` in `.packit.yaml`) deliberately receive none
of these secrets — per the owner ruling of 2026-11-16 the public ranch
must not be able to decrypt anything — so they run without the served
settings file and without credentials, and the credentialed
external-Candlepin tests skip by design while the non-credentialed
tests still run (108 passed / 75 skipped on this head's public-ranch
runs). Of the three Packit jobs, only the internal-ranch job
(`integration-tests/rhel`, submitted with the Packit Production token)
receives them; OSCI gating and team-submitted runs receive them with
their own tokens.

### Local Candlepin

You can also deploy your local candlepin server using
[Unofficial Candlepin Container](https://github.com/candlepin/candlepin-container-unofficial)

```
$ podman run -d --name candlepin \
  -p 8080:8080 -p 8443:8443 --hostname candlepin.local \
  --pull newer ghcr.io/candlepin/candlepin-unofficial:latest
```

When local candlepin is deployed, then you have to do two things. First, you have
to add a `[local]` section to your `.secrets.toml` file exactly like this to use
your local candlepin (a `.secrets.toml` section overrides the downloaded
`settings.toml` and is git-ignored):

```toml
[local]
candlepin.host = "localhost"
candlepin.port =  8443
candlepin.prefix = "/candlepin"
candlepin.insecure = true
candlepin.username =  "mickey"
candlepin.password = "password"
candlepin.activation_keys = ["awesome_os_pool"]
candlepin.org = "donaldduck"
```

### Fake Insights Client

Second thing, you have to do is to create mock of `insights-client`, because real
`insights-client` would try to connect to "insights" server, which is not what you
want for several reasons. First, it is not possible to deploy local "insights" server.
Second, you do not want to use real `insights-client`, because it is so slow. Last
but not least, you do not want to test `insights-client`. You want to test `rhc` here.
Thus, mock of `/usr/bin/insights-client` should look like this:

```bash
#!/bin/bash

if [[ -f /etc/pki/consumer/cert.pem ]]
then
        echo "registered"
        touch /etc/insights-client/.registered
        rm -f /etc/insights-client/.unregistered
        exit 0
else
        echo "not registered"
        touch /etc/insights-client/.unregistered
        rm -f /etc/insights-client/.registered
        exit 1
fi
```

### Yggdrasil Service

When local candlepin server is used, then it is important to modify configuration of
yggdrasil service to not use default MQTT server. It is recommended to configure yggdrasil
to not use MQTT at all or deploy local MQTT server.

In the case you do not want to use MQTT at all, you will have to modify
`/etc/yggdrasil/config.toml` to have the following content:

```toml
# yggdrasil global configuration settings
protocol = "none"
log-level = "debug"
```

Second option (deploy local MQTT server) makes sense in the case you would also like to
use this system also for testing yggdrasil server. You will have to install
[Mosquitto](https://mosquitto.org/) MQTT server and start the service:

```
$ sudo dnf install -y mosquitto
$ sudo systemctl start mosquitto.service
```

Then you can configure your yggdrasil to use local MQTT server 

```toml
# yggdrasil global configuration settings
protocol = "mqtt"
server = ["tcp://localhost:1883"]
log-level = "debug"
```

Running Integration Tests Using pytest
--------------------------------------

When all this is installed and configured, then have to specify your testing
environment defined in `settings.toml` file:

```
export ENV_FOR_DYNACONF=local
```

Then you finally can run test using:

```
pytest -s -vvv --log-level=DEBUG
```

### SELinux AVC collection

`pytest-client-tools` provides the single autouse `check_avcs` fixture for
integration tests. When SELinux, auditd, and the audit tools are available, it
collects each test's audit window with `aureport --avc --interpret`, writes the
report to that test's `artifacts/.../selinux.log` when AVCs are found, and fails
the test for denials not listed as known exceptions. AVC query errors also fail
the test.

RHC's known exceptions live in `integration-tests/rhc_selinux.py` and are
provided through the `client_tools_avc_skips` fixture in
`integration-tests/conftest.py`. The `rhsmcertd_t` rule tracks the audit noise
noted in [CCT-2900](https://redhat.atlassian.net/browse/CCT-2900); the
`auditctl_t` rule is visible only on RHEL 10 and tracks
[RHEL-246545](https://redhat.atlassian.net/browse/RHEL-246545). The shared
fixture disables AVC collection when SELinux or its audit prerequisites are
unavailable.

Tests that produce expected AVCs can request `check_avcs` and adjust the
current test's checks before teardown. Use `skip_all_avcs()` to skip every AVC
collected for that test, or `skip_avc_entry_by_fields()` to skip only entries
matching selected fields. For example:

```python
def test_operation_with_known_unrelated_denials(check_avcs):
    check_avcs.skip_all_avcs()
```

Running Integration Tests Using tmt
-----------------------------------

You can also try to run integration tests using [tmt](https://github.com/teemtee/tmt). You
also need to have `settings.toml` containing `[prod]` section:

Testing specific PR could look like this:

```
export PR_ID="158"
tmt --root . -vvv run --all --keep --environment ENV_FOR_DYNACONF=prod \
  --environment ghprbPullId=${PR_ID} provision --how beaker \
  --image RHEL-10.0-20241022.0 --arch x86_64
```

Testing of main branch:

```
tmt --root . -vvv  run -a --keep --environment ENV_FOR_DYNACONF=prod \
    provision --how beaker --image RHEL-10.0-20241022.0 --arch x86_64
```
