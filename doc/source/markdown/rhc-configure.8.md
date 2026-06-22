% rhc-configure 8

# NAME

rhc-configure - Configure RHC features before or after registration

# SYNOPSIS

```
rhc configure features status
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
: Red Hat Lightspeed data collection. Enables insights-client for system analytics and recommendations.

**remote-management**
: Red Hat Lightspeed remote management capabilities. Enables yggdrasil service for remote system management. Depends on the content and analytics features.

# SUBCOMMANDS

**status**
: List all available features and their current configuration. When disconnected, the table shows PREFERENCE. When connected, the table shows STATE. DESCRIPTION is always included.

**enable** FEATURE
: Enable the specified feature. This sets the user preference and, if the system is already connected, activates the feature on the system. Dependencies are automatically enabled. Multiple features can be configured with a single command.

**disable** FEATURE
: Disable the specified feature. This sets the user preference and, if the system is already connected, deactivates the feature on the system. Dependent features are automatically disabled. Multiple features can be configured with a single command.

# FEATURE STATES

The **status** subcommand shows one of two columns for each feature, depending on whether the system is connected to Red Hat:

**PREFERENCE**
: Shown when the system is disconnected. Indicates the user's saved preference on connect. **enable** indicates that the feature is included on connect, **skip** indicates the feature is excluded on connect.

**STATE**
: Shown when the system is connected. Indicates the current system status. **enabled** indicates the feature is active and its services are running, **disabled** indicates that the feature is disabled and its services are not running.

# FEATURE DEPENDENCIES

Features have the following dependency hierarchy:

- **remote-management** requires **content** and **analytics**
- **analytics** is an independent base feature and has no dependencies
- **content** is an independent base feature and has no dependencies

When enabling a feature, the required dependencies are automatically enabled. When disabling a feature, the dependent features are automatically disabled.

# PRE-REGISTRATION USAGE

Before running **rhc connect**, you can configure feature preferences.

**Note:** The commands shown below are examples to demonstrate the functionality. Pre-configuring features is optional and not required for connecting to Red Hat services. You can run **rhc connect** directly without any prior configuration.

```
# rhc configure features status
FEATURE            PREFERENCE  DESCRIPTION
content            enable      Red Hat content management
analytics          enable      Red Hat Lightspeed data collection
remote-management  enable      Red Hat Lightspeed remote management

# rhc configure features disable analytics
During registration, 'remote-management' will not be enabled (depends on 'analytics').
During registration, 'analytics' will not be enabled.

# rhc connect
```

When you subsequently run **rhc connect**, only the features marked with PREFERENCE **enable** are activated on connect.

# POST-REGISTRATION USAGE

After the system is connected, you can change features dynamically:

```
# rhc configure features status
FEATURE            STATE    DESCRIPTION
content            enabled  Red Hat content management
analytics          enabled  Red Hat Lightspeed data collection
remote-management  enabled  Red Hat Lightspeed remote management

# rhc configure features disable analytics
Feature 'remote-management' disabled (depends on 'analytics').
Feature 'analytics' disabled.

# rhc configure features enable analytics
Feature 'analytics' enabled.
```

Changes take effect immediately on the system by enabling or disabling the underlying services (rhsm, insights-client, yggdrasil).

# CONFIGURATION FILE

Feature preferences are stored in a configuration file at:

```
/var/lib/rhc/rhc-connect-features-prefs.json
```

The file contains JSON booleans tracking connect-time feature preferences:

```
{
    "analytics": true,
    "content": false,
    "remote-management": false
}
```

**true** means the feature is included on connect (shown as **enable** in **status** output); **false** means it is excluded (shown as **skip** in **status** output). Valid keys are **content**, **analytics**, and **remote-management**.

The configuration file can be edited manually, though using **rhc configure** is recommended to ensure dependency consistency. If no preferences file exists, all features default to true (shown as **enable** in status output).

# LIMITATIONS

**rhc configure** performs a one-time configuration change and does not proactively monitor or correct feature states. If an underlying service (such as rhsm, insights-client or yggdrasil) crashes or is manually disabled, **rhc** will not automatically re-enable it.

The command cannot change the server a system is connected to (ConsoleDot vs Satellite) without re-registration. Use **rhc disconnect** and **rhc connect** for that purpose.

# EXAMPLES

**Show current feature configuration:**

```
rhc configure features status
```

**Disable analytics before registration:**

```
rhc configure features disable analytics
rhc connect
```

**Disable content and analytics before registration:**

```
rhc configure features disable content analytics
rhc connect
```

**Enable content and analytics before registration:**

```
rhc configure features enable content analytics
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
- Invalid feature name or wrong argument count provided
- System permission errors when modifying configuration or services

# SEE ALSO

**rhc(1)**, **rhc-connect(8)**, **rhc-disconnect(8)**, **subscription-manager(8)**, **insights-client(8)**
