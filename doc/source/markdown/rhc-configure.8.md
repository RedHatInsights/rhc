% rhc-configure 8

# NAME

rhc-configure - Configure RHC features before or after registration

# SYNOPSIS

```
rhc configure features show
rhc configure features enable FEATURE
rhc configure features disable FEATURE
```

# DESCRIPTION

The **rhc configure** command allows users to manage feature levels and individual features on a host, both before and after registration with Red Hat services. This provides a user-friendly abstraction for changing the host's connectivity level without requiring system re-registration.

Features can be managed independently, allowing users to enable or disable specific functionality such as content access, analytics data collection, and remote management capabilities.

# FEATURES

The following features are available for configuration:

**content**
: Content and repository configuration. This is the base feature that controls access to package repositories.

**analytics**
: Red Hat Lightspeed data collection. Enables insights-client for system analytics and recommendations. Depends on the content feature.

**remote-management**
: Red Hat Lightspeed remote management capabilities. Enables yggdrasil service for remote system management. Depends on the analytics feature.

# SUBCOMMANDS

**show**
: List all available features with their current configuration state and runtime state. The output displays a table with columns for FEATURE, CONFIG (user preference), STATE (actual system state), and DESCRIPTION.

**enable** FEATURE
: Enable the specified feature. This sets the user preference and, if the system is already connected, activates the feature on the system. Dependencies are automatically enabled.

**disable** FEATURE
: Disable the specified feature. This sets the user preference and, if the system is already connected, deactivates the feature on the system. Dependent features are automatically disabled.

# FEATURE STATES

The **show** subcommand displays two states for each feature:

**CONFIG**
: Indicates the user's preference (configuration). A checkmark (✓) means the feature is configured to be enabled, an x means it is configured to be disabled.

**STATE**
: Indicates the actual runtime state of the feature on the system. A checkmark (✓) means the feature is currently active, an x means it is not active.

Before registration, the STATE column shows x for all features since no services are running yet. After registration, STATE reflects whether the underlying services are actually enabled and running.

# FEATURE DEPENDENCIES

Features have the following dependency hierarchy:

- **remote-management** requires **analytics**
- **analytics** requires **content**
- **content** is the base feature with no dependencies

When enabling a feature, all required dependencies are automatically enabled. When disabling a feature, all dependent features are automatically disabled.

# PRE-REGISTRATION USAGE

Before running **rhc connect**, you can configure feature preferences.

**Note:** The commands shown below are examples to demonstrate the functionality. Pre-configuring features is optional and not required for connecting to Red Hat services. You can run **rhc connect** directly without any prior configuration.

```
# rhc configure features show
FEATURE            CONFIG  STATE  DESCRIPTION
content            ✓       x      Access to package repositories
analytics          ✓       x      Red Hat Lightspeed data collection
remote-management  ✓       x      Red Hat Lightspeed remote management

# rhc configure features disable analytics
During registration, analytics will not be enabled.
During registration, remote management will not be enabled.

# rhc connect
```

When you subsequently run **rhc connect**, only the features marked as enabled in CONFIG will be activated.

# POST-REGISTRATION USAGE

After the system is connected, you can change features dynamically:

```
# rhc configure features show
FEATURE            CONFIG  STATE  DESCRIPTION
content            ✓       ✓      Access to package repositories
analytics          ✓       ✓      Red Hat Lightspeed data collection
remote-management  ✓       ✓      Red Hat Lightspeed remote management

# rhc configure features disable analytics
Disabling remote management (depends on analytics).
Disabling analytics.

# rhc configure features enable analytics
Enabling analytics.
```

Changes take effect immediately on the system by enabling or disabling the underlying services (insights-client, yggdrasil).

# CONFIGURATION FILE

Feature preferences are stored in a configuration file at:

```
/etc/rhc/config.toml.d/01-features.toml
```

The file contains a TOML section tracking feature states:

```
[...]
features = {
    "content" = true,
    "analytics" = true,
    "remote-management" = false
}
```

The configuration file can be edited manually, though using **rhc configure** is recommended to ensure dependency consistency.

# LIMITATIONS

**rhc configure** performs a one-time configuration change and does not proactively monitor or correct feature states. If an underlying service (such as insights-client or yggdrasil) crashes or is manually disabled, **rhc** will not automatically re-enable it.

The command cannot change the server a system is connected to (ConsoleDot vs Satellite) without re-registration. Use **rhc disconnect** and **rhc connect** for that purpose.

# EXAMPLES

**Show current feature configuration:**

```
rhc configure features show
```

**Disable analytics before registration:**

```
rhc configure features disable analytics
rhc connect
```

**Enable remote management on a connected system:**

```
rhc configure features enable remote-management
```

**Disable all optional features, keeping only content access:**

```
rhc configure features disable analytics
```

# EXIT STATUS

**0**: Success

**Non-zero**: An error occurred. Common errors include:
- Attempting to enable a feature while its dependencies are disabled
- Attempting to enable and disable the same feature simultaneously
- System permission errors when modifying configuration or services

# SEE ALSO

**rhc(1)**, **rhc-connect(8)**, **rhc-disconnect(8)**, **subscription-manager(8)**, **insights-client(8)**
