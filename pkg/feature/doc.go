/*
Package feature provides a unified interface for managing feature levels.

# System and feature lifecycle

  - Before registration: Use the prefcache subpackage. No system changes are
    applied yet.
  - During and after registration: Features are enabled and disabled
    immediately by altering the system configuration.

Features declare dependencies on other features. Dependencies are enforced:
  - before a feature is enabled, all required features will be enabled,
  - before a feature is disabled, all dependent features will be disabled.

# Package usage

Every feature must implement the IFeature interface. Several feature levels
are available:
  - content: Red Hat content management
  - analytics: Red Hat Lightspeed data collection
  - remote-management: Red Hat Lightspeed remote management

Feature objects can be retrieved using Get() or MustGet():

	feature, err := feature.Get("analytics")
	if err != nil {
		// handle error
	}
	if err := feature.Enable(); err != nil {
		// handle error
	}
*/
package feature
