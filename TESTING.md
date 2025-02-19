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
sudo pip -r install integration-tests/requirements.txt
```

Configuration of Testing Environment
------------------------------------

Then you will have to create configuration file `settings.toml` for your testing. You
will have to replace credentials with your own credentials.

```toml
[production]
candlepin.host = "subscription.rhsm.redhat.com"
candlepin.port =  443
candlepin.prefix = "/subscription"
candlepin.insecure = false
candlepin.username =  "YOUR_USERNAME"
candlepin.password = "YOUR_SECRET_PASSWORD"
candlepin.activation_keys = ["YOR_ACTIVATION_KEY"]
candlepin.org = "YOUR_RORGANIZATION_ID"

# # Following options are optional.
# insights.legacy_upload = false
# insights.host_details = "/var/lib/insights/host-details.json"
# auth_proxy.host = "YOUR.PROXY.SERVER.USING.AUTHENTICATION.COM"
# auth_proxy.port = 3127
# auth_proxy.username = "YOUR_PROXY_USERNAME"
# auth_proxy.password = "YOUR_PROXY_PASSWORD"
# noauth_proxy.host = "YOUR.PROXY.SERVER.NO-AUTHENTICATION"
# noauth_proxy.port = 3129
```

Previous example of configuration file will use production candlepin server.
You can have more section in this configuration file. Another section could be for
example for some Satellite server.

### Local Candlepin

You can also deploy your local candlepin server using
[Pino's container image](https://github.com/ptoscano/candlepin-container-unofficial):

```
$ podman run -d --name candlepin -p 8080:8080 \
  --pull newer ghcr.io/ptoscano/candlepin-unofficial:latest
```

When local candlepin is deployed, then you have to do two things. First, you have
to add configuration to your `settings.toml` file exactly like this to use your
local candlepin:

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
else
	echo "not registered"
	touch /etc/insights-client/.unregistered
	rm -f /etc/insights-client/.registered
fi

exit 0
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
