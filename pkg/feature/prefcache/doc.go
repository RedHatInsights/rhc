/*
Package prefcache keeps track of feature preferences.

The PreferenceCache keeps track of feature preferences in memory and
synchronizes them to filesystem when needed, using a dirty flag to track
state.

	│ NewDefaultCache()
	│ LoadCache()
	▼
	┌──────────────┐───────────>┌──────────────┐
	│ not dirty    │ Set()      │ dirty        │
	│ file absent  │   Delete() │ file absent  │
	└──────────────┘<───────────└──────────────┘
	▲                                   Save() │
	│ Delete()                                 ▼
	┌──────────────┐───────────>┌──────────────┐
	│ dirty        │ Save()     │ not dirty    │
	│ file present │      Set() │ file present │
	└──────────────┘<───────────└──────────────┘
	                                           ▲
	                               LoadCache() │

This diagram omits parts of the internal behavior:

  - Set does not set a dirty flag if nothing changed (i.e., enabled feature is
    requested to be enabled).
  - Save does not write down the cache if it is not dirty.
  - Save deletes the file if the cache matches the default cache.
*/
package prefcache
