package subman

// Organization contains information about an RHSM organization (Candlepin owner).
type Organization struct {
	ID                         string  `json:"id"`
	Key                        string  `json:"key"`
	DisplayName                *string `json:"displayName,omitempty"`
	Created                    *string `json:"created,omitempty"`
	Updated                    *string `json:"updated,omitempty"`
	ContentPrefix              *string `json:"contentPrefix,omitempty"`
	DefaultServiceLevel        *string `json:"defaultServiceLevel,omitempty"`
	LogLevel                   *string `json:"logLevel,omitempty"`
	ContentAccessMode          *string `json:"contentAccessMode,omitempty"`
	ContentAccessModeList      *string `json:"contentAccessModeList,omitempty"`
	AutobindHypervisorDisabled *bool   `json:"autobindHypervisorDisabled,omitempty"`
	AutobindDisabled           *bool   `json:"autobindDisabled,omitempty"`
	LastRefreshed              *string `json:"lastRefreshed,omitempty"`
}

// Environment contains information about a Candlepin environment.
type Environment struct {
	ID                 string               `json:"id"`
	Created            *string              `json:"created,omitempty"`
	Updated            *string              `json:"updated,omitempty"`
	Name               *string              `json:"name,omitempty"`
	Type               *string              `json:"type,omitempty"`
	Description        *string              `json:"description,omitempty"`
	ContentPrefix      *string              `json:"contentPrefix,omitempty"`
	Owner              *EnvironmentOwner      `json:"owner,omitempty"`
	EnvironmentContent []EnvironmentContent `json:"environmentContent,omitempty"`
}

// EnvironmentOwner contains abbreviated owner information on an environment.
type EnvironmentOwner struct {
	ID                string  `json:"id"`
	Key               *string `json:"key,omitempty"`
	DisplayName       *string `json:"displayName,omitempty"`
	Href              *string `json:"href,omitempty"`
	ContentAccessMode *string `json:"contentAccessMode,omitempty"`
}

// EnvironmentContent describes content enabled in an environment.
type EnvironmentContent struct {
	ContentID string `json:"contentId"`
	Enabled   bool   `json:"enabled"`
}
