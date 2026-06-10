package main

import (
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/redhatinsights/rhc/internal/canonical_facts"
)

const (
	providerID = "com.redhat.advisor"
	reporter   = "rhc-data-collector"
)

// runDataCollection executes the complete data collection process.
func runDataCollection(outputDir string) error {
	// Collect canonical facts
	facts, err := canonical_facts.GetCanonicalFacts()
	if err != nil {
		return err
	}

	// Create HBI ingress message
	hbiMessage := createHBIMessage(facts)
	if err := writeJSONFile(outputDir, "host.json", hbiMessage); err != nil {
		return err
	}

	// Create canonical facts file for compatibility
	if err := writeJSONFile(outputDir, "canonical_facts.json", facts); err != nil {
		return err
	}

	// Create archive metadata
	archiveMetadata := createArchiveMetadata(facts)
	if err := writeJSONFile(outputDir, "archive_metadata.json", archiveMetadata); err != nil {
		return err
	}

	slog.Debug("Generated files", "host", "host.json", "canonical_facts", "canonical_facts.json", "archive_metadata", "archive_metadata.json")
	return nil
}

// createHBIMessage generates the HBI ingress message format.
//
// See: https://inscope.corp.redhat.com/docs/default/component/host-based-inventory.
func createHBIMessage(facts *canonical_facts.CanonicalFacts) map[string]interface{} {
	// Use machine_id as fallback when insights_id is missing
	insightsID := facts.InsightsID
	if insightsID == "" {
		insightsID = facts.MachineID
	}

	// Extract org_id from RHSM certificate
	orgID, err := extractOrgID("/etc/pki/consumer/cert.pem")
	if err != nil {
		slog.Debug("Failed to extract org_id from RHSM certificate", "error", err)
		orgID = "unknown"
	}

	return map[string]interface{}{
		"operation": "add_host",
		"operation_args": map[string]interface{}{
			"defer_to_reporter": reporter,
		},
		"platform_metadata": map[string]interface{}{
			"request_id": reporter + "-" + facts.MachineID,
		},
		"data": map[string]interface{}{
			"org_id":                  orgID,
			"reporter":                reporter,
			"provider_id":             providerID,
			"provider_type":           "rhc",
			"insights_id":             insightsID,
			"machine_id":              facts.MachineID,
			"bios_uuid":               facts.BIOSUUID,
			"subscription_manager_id": facts.SubscriptionManagerID,
			"ip_addresses":            facts.IPAddresses,
			"mac_addresses":           facts.MACAddresses,
			"fqdn":                    facts.FQDN,
		},
	}
}

// createArchiveMetadata generates the archive metadata.
func createArchiveMetadata(facts *canonical_facts.CanonicalFacts) map[string]interface{} {
	return map[string]interface{}{
		"provider_id":     providerID,
		"collection_type": "ingress",
		"collector":       "data-collector",
		"canonical_facts": facts,
	}
}

// writeJSONFile writes data to a JSON file with pretty formatting.
func writeJSONFile(outputDir, filename string, data interface{}) error {
	filePath := filepath.Join(outputDir, filename)
	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer func(file *os.File) {
		if err := file.Close(); err != nil {
			slog.Debug("failed to close file", "file", file.Name(), "error", err)
			return
		}
	}(file)

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(data)
}

// extractOrgID reads the RHSM consumer certificate and extracts the org_id.
func extractOrgID(filename string) (string, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return "", err
	}

	block, _ := pem.Decode(data)
	if block == nil {
		return "", fmt.Errorf("failed to decode PEM data: %v", filename)
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return "", err
	}

	// Extract org ID from Organization field
	if len(cert.Subject.Organization) > 0 {
		return cert.Subject.Organization[0], nil
	}

	return "", fmt.Errorf("no organization found in certificate")
}
