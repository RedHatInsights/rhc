package main

import (
	"cmp"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/redhatinsights/rhc/internal/canonical_facts"
	"github.com/redhatinsights/rhc/pkg/version"
)

// Satellite management constants. Both set to -1 to indicate "not managed by Satellite".
// When puptoo processes branch_info, it checks: if remote_branch != -1, it sets
// satellite_managed=true and satellite_id=remote_leaf.
const (
	remoteBranch = -1
	remoteLeaf   = -1
)

// MetadataSpec represents metadata for a collected spec that is written to
// the meta_data/ directory in insights archives. These metadata files are
// required by insights-core's hydration process (serde.py) to load and parse
// the archive data. The metadata format follows the structure created by
// insights-client when it collects data using insights-core's spec framework.
type MetadataSpec struct {
	// Name is the fully qualified spec name in insights-core's namespace.
	Name string `json:"name"`

	// ExecTime is the execution time in seconds for collecting this spec.
	// For minimal collector, use a placeholder value like 0.0001.
	ExecTime float64 `json:"exec_time"`

	// Errors is a list of error messages encountered during collection.
	Errors []string `json:"errors"`

	// Results contains the spec type and location information.
	// Structure: {"type": "<provider_type>", "object": {"relative_path": "...", ...}}
	// Provider types:
	//   - insights.core.spec_factory.TextFileProvider (for file content)
	//   - insights.core.spec_factory.CommandOutputProvider (for command output)
	//   - insights.core.spec_factory.DatasourceProvider (for computed data)
	// The "relative_path" field points to the actual data file in data/ directory.
	Results any `json:"results"`

	// SerTime is the serialization time in seconds for this spec's results.
	// For minimal collector, use a placeholder value like 0.0001.
	SerTime float64 `json:"ser_time"`
}

// SpecObject represents the "object" field within MetadataSpec.Results.
// This structure describes how insights-core should locate and process the data file.
type SpecObject struct {
	// RelativePath is the path to the data file relative to the data/ directory.
	RelativePath string `json:"relative_path"`

	// SaveAs specifies an alternate path to save the file during collection.
	// For minimal collector, always nil.
	SaveAs any `json:"save_as"`

	// RC is the command exit code (for CommandOutputProvider) or nil (for file-based providers).
	// When a command is executed during collection, this stores the exit code.
	// For minimal collector, always nil.
	RC any `json:"rc"`

	// Cmd is the original command string (for CommandOutputProvider only).
	// For minimal collector, always nil.
	Cmd any `json:"cmd"`

	// Args are the command arguments (for CommandOutputProvider only).
	// For minimal collector, always nil.
	Args any `json:"args"`
}

// VersionInfo represents the version_info file content.
type VersionInfo struct {
	CoreVersion   string  `json:"core_version"`
	ClientVersion *string `json:"client_version"`
}

// BranchInfo represents the branch_info file content.
type BranchInfo struct {
	RemoteBranch int `json:"remote_branch"`
	RemoteLeaf   int `json:"remote_leaf"`
}

// createArchiveStructure creates an insights-core compatible archive structure.
func createArchiveStructure(outputDir string) (string, string, error) {
	dataDir := filepath.Join(outputDir, "data")
	metaDataDir := filepath.Join(outputDir, "meta_data")

	if err := os.MkdirAll(dataDir, 0700); err != nil {
		return "", "", fmt.Errorf("failed to create data directory: %w", err)
	}
	if err := os.MkdirAll(metaDataDir, 0700); err != nil {
		return "", "", fmt.Errorf("failed to create meta_data directory: %w", err)
	}
	// insights_archive.txt is an empty file used by insights-core to identify this as
	// a SerializedArchiveContext. Its presence indicates this is an insights archive.
	archiveMarker := filepath.Join(outputDir, "insights_archive.txt")
	if err := os.WriteFile(archiveMarker, []byte{}, 0600); err != nil {
		return "", "", fmt.Errorf("failed to write insights_archive.txt: %w", err)
	}

	return dataDir, metaDataDir, nil
}

// getOrgID retrieves the organization ID from the subscription-manager certificate.
// The org_id is stored in the Organization field (O) of the certificate Subject.
func getOrgID() (string, error) {
	certPath := "/etc/pki/consumer/cert.pem"
	data, err := os.ReadFile(certPath)
	if err != nil {
		slog.Error("failed to read consumer cert", "path", certPath, "error", err)
		return "", fmt.Errorf("failed to read consumer cert: %w", err)
	}

	block, _ := pem.Decode(data)
	if block == nil {
		slog.Error("failed to decode PEM data from consumer cert", "path", certPath)
		return "", fmt.Errorf("failed to decode PEM data from %s", certPath)
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		slog.Error("failed to parse certificate from consumer cert", "path", certPath, "error", err)
		return "", fmt.Errorf("failed to parse certificate: %w", err)
	}

	if len(cert.Subject.Organization) == 0 {
		slog.Error("failed to parse organization from consumer cert", "path", certPath)
		return "", fmt.Errorf("failed to parse organization from %s", certPath)
	}

	orgID := cert.Subject.Organization[0]
	slog.Debug("found org ID from certificate", "org_id", orgID)
	return orgID, nil
}

// writeVersionInfo creates version_info file and metadata.
//
// The version_info file contains collector version information that Puptoo extracts
// and includes in the system profile sent to HBI:
//   - core_version → system_profile.insights_egg_version
//   - client_version → system_profile.insights_client_version
//
// These fields help track which collector version generated the archive, useful for
// debugging data collection issues and understanding feature availability across hosts.
// For minimal collector, core_version is set to rhc's version and client_version is null
// since there's no separate insights-client component.
func writeVersionInfo(dataDir, metaDataDir string) error {
	versionInfo := VersionInfo{
		CoreVersion:   version.Version,
		ClientVersion: nil,
	}

	versionData, err := json.Marshal(versionInfo)
	if err != nil {
		return fmt.Errorf("failed to marshal version_info: %w", err)
	}

	versionPath := filepath.Join(dataDir, "version_info")
	if err = os.WriteFile(versionPath, versionData, 0600); err != nil {
		return fmt.Errorf("failed to write version_info: %w", err)
	}

	// Write metadata
	metadata := MetadataSpec{
		Name:     "insights.specs.Specs.version_info",
		ExecTime: 0.0001,
		Errors:   []string{},
		Results: map[string]any{
			"type": "insights.core.spec_factory.DatasourceProvider",
			"object": SpecObject{
				RelativePath: "version_info",
				SaveAs:       nil,
				RC:           nil,
				Cmd:          nil,
				Args:         nil,
			},
		},
		SerTime: 0.0001,
	}

	metaData, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal version_info metadata: %w", err)
	}

	metaPath := filepath.Join(metaDataDir, "insights.specs.Specs.version_info.json")
	if err := os.WriteFile(metaPath, metaData, 0600); err != nil {
		return fmt.Errorf("failed to write version_info metadata: %w", err)
	}

	return nil
}

// writeBranchInfo creates branch_info file and metadata. Puptoo uses this to
// determine Satellite management: if remote_branch != -1, sets satellite_managed=true
// and satellite_id=remote_leaf. For minimal collector, both values are -1.
func writeBranchInfo(dataDir, metaDataDir string) error {
	branchInfo := BranchInfo{
		RemoteBranch: remoteBranch,
		RemoteLeaf:   remoteLeaf,
	}

	branchData, err := json.Marshal(branchInfo)
	if err != nil {
		return fmt.Errorf("failed to marshal branch_info: %w", err)
	}

	branchPath := filepath.Join(dataDir, "branch_info")
	if err := os.WriteFile(branchPath, branchData, 0600); err != nil {
		return fmt.Errorf("failed to write branch_info: %w", err)
	}

	// Write metadata
	metadata := MetadataSpec{
		Name:     "insights.specs.Specs.branch_info",
		ExecTime: 0.0001,
		Errors:   []string{},
		Results: map[string]any{
			"type": "insights.core.spec_factory.DatasourceProvider",
			"object": SpecObject{
				RelativePath: "branch_info",
				SaveAs:       nil,
				RC:           nil,
				Cmd:          nil,
				Args:         nil,
			},
		},
		SerTime: 0.0001,
	}

	metaData, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal branch_info metadata: %w", err)
	}

	metaPath := filepath.Join(metaDataDir, "insights.specs.Specs.branch_info.json")
	if err := os.WriteFile(metaPath, metaData, 0600); err != nil {
		return fmt.Errorf("failed to write branch_info metadata: %w", err)
	}

	return nil
}

// writeCanonicalFacts collects and writes canonical facts to the archive.
func writeCanonicalFacts(dataDir, metaDataDir string) error {
	facts, err := canonical_facts.GetCanonicalFacts()
	if err != nil {
		slog.Error("failed to get canonical facts", "error", err)
		return fmt.Errorf("failed to get canonical facts: %w", err)
	}

	// Create subdirectories
	etcDir := filepath.Join(dataDir, "etc", "insights-client")
	commandsDir := filepath.Join(dataDir, "insights_commands")

	for _, dir := range []string{etcDir, commandsDir} {
		if err = os.MkdirAll(dir, 0700); err != nil {
			slog.Error("failed to create directory", "path", dir, "error", err)
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	if facts.InsightsID != "" {
		if err = writeInsightsID(etcDir, metaDataDir, facts.InsightsID); err != nil {
			return err
		}
	}

	if facts.SubscriptionManagerID != "" {
		if err = writeSubscriptionManagerID(commandsDir, metaDataDir, facts.SubscriptionManagerID); err != nil {
			return err
		}
	}

	if len(facts.IPAddresses) > 0 {
		if err = writeIPAddresses(commandsDir, metaDataDir, facts.IPAddresses); err != nil {
			return err
		}
	}

	if len(facts.MACAddresses) > 0 {
		if err = writeMACAddresses(dataDir, metaDataDir); err != nil {
			return err
		}
	}

	if facts.BIOSUUID != "" {
		if err = writeBIOSUUID(commandsDir, metaDataDir, facts.BIOSUUID); err != nil {
			return err
		}
	}

	if facts.FQDN != "" {
		if err = writeFQDN(commandsDir, metaDataDir, facts.FQDN); err != nil {
			return err
		}
	}

	if facts.MachineID != "" {
		if err = writeMachineID(filepath.Join(dataDir, "etc"), metaDataDir, facts.MachineID); err != nil {
			return err
		}
	}

	return nil
}

// writeInsightsID writes the insights-client machine-id.
func writeInsightsID(etcDir, metaDataDir, insightsID string) error {
	path := filepath.Join(etcDir, "machine-id")
	if err := os.WriteFile(path, []byte(insightsID), 0600); err != nil {
		slog.Error("failed to write insights machine-id to file", "path", path, "error", err)
		return fmt.Errorf("failed to write insights machine-id to %s: %w", path, err)
	}

	metadata := MetadataSpec{
		Name:     "insights.specs.Specs.machine_id",
		ExecTime: 0.0001,
		Errors:   []string{},
		Results: map[string]any{
			"type": "insights.core.spec_factory.TextFileProvider",
			"object": SpecObject{
				RelativePath: "etc/insights-client/machine-id",
				SaveAs:       nil,
				RC:           nil,
				Cmd:          nil,
				Args:         nil,
			},
		},
		SerTime: 0.0001,
	}

	return writeMetadata(metaDataDir, "insights.specs.Specs.machine_id.json", metadata)
}

// writeSubscriptionManagerID writes subscription-manager identity output.
func writeSubscriptionManagerID(commandsDir, metaDataDir, subMgrID string) error {
	orgID, err := getOrgID()
	if err != nil {
		slog.Error("failed to get org_id for subscription-manager output", "error", err)
		return fmt.Errorf("failed to get org_id for subscription-manager output: %w", err)
	}

	hostname, err := os.Hostname()
	if err != nil {
		slog.Warn("failed to get hostname for subscription-manager output", "error", err)
		hostname = "unknown"
	}

	output := fmt.Sprintf("system identity: %s\nname: %s\norg name: %s\norg ID: %s\n",
		subMgrID, hostname, orgID, orgID)

	path := filepath.Join(commandsDir, "subscription-manager_identity")
	if err := os.WriteFile(path, []byte(output), 0600); err != nil {
		slog.Error("failed to write subscription-manager identity to file", "path", path, "error", err)
		return fmt.Errorf("failed to write subscription-manager identity to %s: %w", path, err)
	}

	metadata := MetadataSpec{
		Name:     "insights.specs.Specs.subscription_manager_id",
		ExecTime: 0.0001,
		Errors:   []string{},
		Results: map[string]any{
			"type": "insights.core.spec_factory.CommandOutputProvider",
			"object": SpecObject{
				RelativePath: "insights_commands/subscription-manager_identity",
				SaveAs:       nil,
				RC:           nil,
				Cmd:          nil,
				Args:         nil,
			},
		},
		SerTime: 0.0001,
	}

	return writeMetadata(metaDataDir, "insights.specs.Specs.subscription_manager_id.json", metadata)
}

// writeIPAddresses writes hostname -I output.
func writeIPAddresses(commandsDir, metaDataDir string, ipAddresses []string) error {
	output := strings.Join(ipAddresses, " ") + "\n"

	path := filepath.Join(commandsDir, "hostname_-I")
	if err := os.WriteFile(path, []byte(output), 0600); err != nil {
		slog.Error("failed to write IP addresses to file", "path", path, "error", err)
		return fmt.Errorf("failed to write IP addresses to %s: %w", path, err)
	}

	metadata := MetadataSpec{
		Name:     "insights.specs.Specs.ip_addresses",
		ExecTime: 0.0001,
		Errors:   []string{},
		Results: map[string]any{
			"type": "insights.core.spec_factory.CommandOutputProvider",
			"object": SpecObject{
				RelativePath: "insights_commands/hostname_-I",
				SaveAs:       nil,
				RC:           nil,
				Cmd:          nil,
				Args:         nil,
			},
		},
		SerTime: 0.0001,
	}

	return writeMetadata(metaDataDir, "insights.specs.Specs.ip_addresses.json", metadata)
}

// writeMACAddresses writes MAC addresses data files and metadata.
func writeMACAddresses(dataDir, metaDataDir string) error {
	// Get network interfaces to match MAC addresses with interface names
	ifaces, err := net.Interfaces()
	if err != nil {
		slog.Error("failed to get network interfaces", "error", err)
		return fmt.Errorf("failed to get network interfaces: %w", err)
	}

	// Sort interfaces by name for consistent ordering
	slices.SortFunc(ifaces, func(a, b net.Interface) int {
		return cmp.Compare(a.Name, b.Name)
	})

	// Build results array with one entry per interface
	results := make([]map[string]any, 0, len(ifaces))
	for _, iface := range ifaces {
		ifaceDir := filepath.Join(dataDir, "sys", "class", "net", iface.Name)
		if err := os.MkdirAll(ifaceDir, 0700); err != nil {
			slog.Error("failed to create interface directory", "path", ifaceDir, "error", err)
			return fmt.Errorf("failed to create directory %s: %w", ifaceDir, err)
		}

		macPath := filepath.Join(ifaceDir, "address")
		macAddr := iface.HardwareAddr.String()
		if err := os.WriteFile(macPath, []byte(macAddr+"\n"), 0600); err != nil {
			slog.Error("failed to write MAC address to file", "path", macPath, "error", err)
			return fmt.Errorf("failed to write MAC address to %s: %w", macPath, err)
		}

		results = append(results, map[string]any{
			"type": "insights.core.spec_factory.TextFileProvider",
			"object": SpecObject{
				RelativePath: fmt.Sprintf("sys/class/net/%s/address", iface.Name),
				SaveAs:       nil,
				RC:           nil,
				Cmd:          nil,
				Args:         nil,
			},
		})
	}

	metadata := MetadataSpec{
		Name:     "insights.specs.Specs.mac_addresses",
		ExecTime: 0.0001,
		Errors:   []string{},
		Results:  results,
		SerTime:  0.0001,
	}

	return writeMetadata(metaDataDir, "insights.specs.Specs.mac_addresses.json", metadata)
}

// writeBIOSUUID writes minimal dmidecode output with UUID.
func writeBIOSUUID(commandsDir, metaDataDir, biosUUID string) error {
	// Create minimal dmidecode-style output containing just the UUID
	output := fmt.Sprintf("# dmidecode 3.99.0\n"+
		"Getting SMBIOS data from sysfs.\n"+
		"\n"+
		"Handle 0x0001, DMI type 1, 27 bytes\n"+
		"System Information\n"+
		"\tUUID: %s\n", biosUUID)

	path := filepath.Join(commandsDir, "dmidecode")
	if err := os.WriteFile(path, []byte(output), 0600); err != nil {
		slog.Error("failed to write dmidecode to file", "path", path, "error", err)
		return fmt.Errorf("failed to write dmidecode to %s: %w", path, err)
	}

	metadata := MetadataSpec{
		Name:     "insights.specs.Specs.dmidecode",
		ExecTime: 0.0001,
		Errors:   []string{},
		Results: map[string]any{
			"type": "insights.core.spec_factory.CommandOutputProvider",
			"object": SpecObject{
				RelativePath: "insights_commands/dmidecode",
				SaveAs:       nil,
				RC:           nil,
				Cmd:          nil,
				Args:         nil,
			},
		},
		SerTime: 0.0001,
	}

	return writeMetadata(metaDataDir, "insights.specs.Specs.dmidecode.json", metadata)
}

// writeFQDN writes hostname output.
func writeFQDN(commandsDir, metaDataDir, fqdn string) error {
	path := filepath.Join(commandsDir, "hostname_-f")
	if err := os.WriteFile(path, []byte(fqdn+"\n"), 0600); err != nil {
		slog.Error("failed to write FQDN to file", "path", path, "error", err)
		return fmt.Errorf("failed to write FQDN to %s: %w", path, err)
	}

	metadata := MetadataSpec{
		Name:     "insights.specs.Specs.hostname",
		ExecTime: 0.0001,
		Errors:   []string{},
		Results: map[string]any{
			"type": "insights.core.spec_factory.CommandOutputProvider",
			"object": SpecObject{
				RelativePath: "insights_commands/hostname_-f",
				SaveAs:       nil,
				RC:           nil,
				Cmd:          nil,
				Args:         nil,
			},
		},
		SerTime: 0.0001,
	}

	return writeMetadata(metaDataDir, "insights.specs.Specs.hostname.json", metadata)
}

// writeMachineID writes system machine-id.
func writeMachineID(etcDir, metaDataDir, machineID string) error {
	path := filepath.Join(etcDir, "machine-id")
	if err := os.WriteFile(path, []byte(machineID), 0600); err != nil {
		slog.Error("failed to write machine-id to file", "path", path, "error", err)
		return fmt.Errorf("failed to write machine-id to %s: %w", path, err)
	}

	metadata := MetadataSpec{
		Name:     "insights.specs.Specs.etc_machine_id",
		ExecTime: 0.0001,
		Errors:   []string{},
		Results: map[string]any{
			"type": "insights.core.spec_factory.TextFileProvider",
			"object": SpecObject{
				RelativePath: "etc/machine-id",
				SaveAs:       nil,
				RC:           nil,
				Cmd:          nil,
				Args:         nil,
			},
		},
		SerTime: 0.0001,
	}

	return writeMetadata(metaDataDir, "insights.specs.Specs.etc_machine_id.json", metadata)
}

// writeMetadata writes a metadata JSON file.
func writeMetadata(metaDataDir, filename string, metadata MetadataSpec) error {
	metaData, err := json.Marshal(metadata)
	if err != nil {
		slog.Error("failed to marshal metadata", "error", err)
		return fmt.Errorf("failed to marshal metadata for %s: %w", filename, err)
	}

	metaPath := filepath.Join(metaDataDir, filename)
	if err = os.WriteFile(metaPath, metaData, 0600); err != nil {
		slog.Error("failed to write metadata to file", "filename", filename, "error", err)
		return fmt.Errorf("failed to write metadata to %s: %w", filename, err)
	}

	slog.Debug("successfully written metadata to file", "metadata", metadata, "filename", filename)
	return nil
}
