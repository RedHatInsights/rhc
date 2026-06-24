package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCreateArchiveStructure(t *testing.T) {
	tmpDir := t.TempDir()

	dataDir, metaDataDir, err := createArchiveStructure(tmpDir)
	if err != nil {
		t.Fatalf("createArchiveStructure failed: %v", err)
	}

	// Verify data directory exists
	if _, err = os.Stat(dataDir); os.IsNotExist(err) {
		t.Errorf("data directory not created: %s", dataDir)
	}

	// Verify meta_data directory exists
	if _, err = os.Stat(metaDataDir); os.IsNotExist(err) {
		t.Errorf("meta_data directory not created: %s", metaDataDir)
	}

	// Verify insights_archive.txt marker exists
	markerPath := filepath.Join(tmpDir, "insights_archive.txt")
	if _, err := os.Stat(markerPath); os.IsNotExist(err) {
		t.Errorf("insights_archive.txt marker not created")
	}

	// Verify insights_archive.txt is empty
	content, err := os.ReadFile(markerPath)
	if err != nil {
		t.Fatalf("failed to read insights_archive.txt: %v", err)
	}
	if len(content) != 0 {
		t.Errorf("insights_archive.txt should be empty, got %d bytes", len(content))
	}
}

func TestWriteMetadata(t *testing.T) {
	tmpDir := t.TempDir()

	metadata := MetadataSpec{
		Name:     "test.spec",
		ExecTime: 0.0001,
		Errors:   []string{},
		Results: map[string]any{
			"type": "test.type",
			"object": SpecObject{
				RelativePath: "test/path",
				SaveAs:       nil,
				RC:           nil,
				Cmd:          nil,
				Args:         nil,
			},
		},
		SerTime: 0.0001,
	}

	err := writeMetadata(tmpDir, "test.spec.json", metadata)
	if err != nil {
		t.Fatalf("writeMetadata failed: %v", err)
	}

	// Verify file exists
	metaPath := filepath.Join(tmpDir, "test.spec.json")
	if _, err := os.Stat(metaPath); os.IsNotExist(err) {
		t.Errorf("metadata file not created: %s", metaPath)
	}

	// Verify content can be parsed
	content, err := os.ReadFile(metaPath)
	if err != nil {
		t.Fatalf("failed to read metadata: %v", err)
	}

	var parsed MetadataSpec
	if err := json.Unmarshal(content, &parsed); err != nil {
		t.Fatalf("failed to parse metadata JSON: %v", err)
	}

	if parsed.Name != "test.spec" {
		t.Errorf("expected name=test.spec, got %s", parsed.Name)
	}
}

func TestObjectSerialization(t *testing.T) {
	obj := SpecObject{
		RelativePath: "etc/machine-id",
		SaveAs:       false,
		RC:           nil,
		Cmd:          nil,
		Args:         nil,
	}

	data, err := json.Marshal(obj)
	if err != nil {
		t.Fatalf("failed to marshal object: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal object: %v", err)
	}

	if parsed["relative_path"] != "etc/machine-id" {
		t.Errorf("expected relative_path=etc/machine-id, got %v", parsed["relative_path"])
	}

	if parsed["save_as"] != false {
		t.Errorf("expected save_as=false, got %v", parsed["save_as"])
	}
}

func TestVersionInfoBranchInfo(t *testing.T) {
	vi := VersionInfo{
		CoreVersion:   "1.0.0",
		ClientVersion: nil,
	}

	data, err := json.Marshal(vi)
	if err != nil {
		t.Fatalf("failed to marshal VersionInfo: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal VersionInfo: %v", err)
	}

	if parsed["core_version"] != "1.0.0" {
		t.Errorf("expected core_version=1.0.0, got %v", parsed["core_version"])
	}

	bi := BranchInfo{
		RemoteBranch: -1,
		RemoteLeaf:   -1,
	}

	data, err = json.Marshal(bi)
	if err != nil {
		t.Fatalf("failed to marshal BranchInfo: %v", err)
	}

	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal BranchInfo: %v", err)
	}

	if parsed["remote_branch"].(float64) != -1 {
		t.Errorf("expected remote_branch=-1, got %v", parsed["remote_branch"])
	}
}

func TestWriteInsightsID(t *testing.T) {
	tmpDir := t.TempDir()
	etcDir := filepath.Join(tmpDir, "etc", "insights-client")
	metaDataDir := filepath.Join(tmpDir, "meta_data")

	if err := os.MkdirAll(etcDir, 0700); err != nil {
		t.Fatalf("failed to create etc dir: %v", err)
	}
	if err := os.MkdirAll(metaDataDir, 0700); err != nil {
		t.Fatalf("failed to create meta_data dir: %v", err)
	}

	testID := "test-insights-id-12345"
	err := writeInsightsID(etcDir, metaDataDir, testID)
	if err != nil {
		t.Fatalf("writeInsightsID failed: %v", err)
	}

	// Verify data file
	dataPath := filepath.Join(etcDir, "machine-id")
	content, err := os.ReadFile(dataPath)
	if err != nil {
		t.Fatalf("failed to read machine-id: %v", err)
	}

	if string(content) != testID {
		t.Errorf("expected content=%s, got %s", testID, string(content))
	}

	// Verify metadata file
	metaPath := filepath.Join(metaDataDir, "insights.specs.Specs.machine_id.json")
	if _, err := os.Stat(metaPath); os.IsNotExist(err) {
		t.Errorf("metadata file not created")
	}
}

func TestWriteIPAddresses(t *testing.T) {
	tmpDir := t.TempDir()
	commandsDir := filepath.Join(tmpDir, "insights_commands")
	metaDataDir := filepath.Join(tmpDir, "meta_data")

	if err := os.MkdirAll(commandsDir, 0700); err != nil {
		t.Fatalf("failed to create commands dir: %v", err)
	}
	if err := os.MkdirAll(metaDataDir, 0700); err != nil {
		t.Fatalf("failed to create meta_data dir: %v", err)
	}

	testIPs := []string{"192.168.1.1", "10.0.0.1"}
	err := writeIPAddresses(commandsDir, metaDataDir, testIPs)
	if err != nil {
		t.Fatalf("writeIPAddresses failed: %v", err)
	}

	// Verify data file
	dataPath := filepath.Join(commandsDir, "hostname_-I")
	content, err := os.ReadFile(dataPath)
	if err != nil {
		t.Fatalf("failed to read hostname_-I: %v", err)
	}

	expected := "192.168.1.1 10.0.0.1\n"
	if string(content) != expected {
		t.Errorf("expected content=%q, got %q", expected, string(content))
	}

	// Verify metadata file
	metaPath := filepath.Join(metaDataDir, "insights.specs.Specs.ip_addresses.json")
	if _, err := os.Stat(metaPath); os.IsNotExist(err) {
		t.Errorf("metadata file not created")
	}
}

func TestWriteFQDN(t *testing.T) {
	tmpDir := t.TempDir()
	commandsDir := filepath.Join(tmpDir, "insights_commands")
	metaDataDir := filepath.Join(tmpDir, "meta_data")

	if err := os.MkdirAll(commandsDir, 0700); err != nil {
		t.Fatalf("failed to create commands dir: %v", err)
	}
	if err := os.MkdirAll(metaDataDir, 0700); err != nil {
		t.Fatalf("failed to create meta_data dir: %v", err)
	}

	testFQDN := "test.example.com"
	err := writeFQDN(commandsDir, metaDataDir, testFQDN)
	if err != nil {
		t.Fatalf("writeFQDN failed: %v", err)
	}

	// Verify data file
	dataPath := filepath.Join(commandsDir, "hostname_-f")
	content, err := os.ReadFile(dataPath)
	if err != nil {
		t.Fatalf("failed to read hostname_-f: %v", err)
	}

	expected := "test.example.com\n"
	if string(content) != expected {
		t.Errorf("expected content=%q, got %q", expected, string(content))
	}

	// Verify metadata file
	metaPath := filepath.Join(metaDataDir, "insights.specs.Specs.hostname.json")
	if _, err := os.Stat(metaPath); os.IsNotExist(err) {
		t.Errorf("metadata file not created")
	}
}

func TestWriteMachineID(t *testing.T) {
	tmpDir := t.TempDir()
	etcDir := filepath.Join(tmpDir, "etc")
	metaDataDir := filepath.Join(tmpDir, "meta_data")

	if err := os.MkdirAll(etcDir, 0700); err != nil {
		t.Fatalf("failed to create etc dir: %v", err)
	}
	if err := os.MkdirAll(metaDataDir, 0700); err != nil {
		t.Fatalf("failed to create meta_data dir: %v", err)
	}

	testMachineID := "abc123-def456-ghi789"
	err := writeMachineID(etcDir, metaDataDir, testMachineID)
	if err != nil {
		t.Fatalf("writeMachineID failed: %v", err)
	}

	// Verify data file
	dataPath := filepath.Join(etcDir, "machine-id")
	content, err := os.ReadFile(dataPath)
	if err != nil {
		t.Fatalf("failed to read machine-id: %v", err)
	}

	if string(content) != testMachineID {
		t.Errorf("expected content=%s, got %s", testMachineID, string(content))
	}

	// Verify metadata file
	metaPath := filepath.Join(metaDataDir, "insights.specs.Specs.etc_machine_id.json")
	if _, err := os.Stat(metaPath); os.IsNotExist(err) {
		t.Errorf("metadata file not created")
	}
}

func TestWriteBIOSUUID(t *testing.T) {
	tmpDir := t.TempDir()
	commandsDir := filepath.Join(tmpDir, "insights_commands")
	metaDataDir := filepath.Join(tmpDir, "meta_data")

	if err := os.MkdirAll(commandsDir, 0700); err != nil {
		t.Fatalf("failed to create commands dir: %v", err)
	}
	if err := os.MkdirAll(metaDataDir, 0700); err != nil {
		t.Fatalf("failed to create meta_data dir: %v", err)
	}

	testUUID := "12345678-1234-1234-1234-123456789012"
	err := writeBIOSUUID(commandsDir, metaDataDir, testUUID)
	if err != nil {
		t.Fatalf("writeBIOSUUID failed: %v", err)
	}

	// Verify data file contains UUID
	dataPath := filepath.Join(commandsDir, "dmidecode")
	content, err := os.ReadFile(dataPath)
	if err != nil {
		t.Fatalf("failed to read dmidecode: %v", err)
	}

	contentStr := string(content)
	if !strings.Contains(contentStr, testUUID) {
		t.Errorf("dmidecode output does not contain UUID %s", testUUID)
	}

	if !strings.Contains(contentStr, "# dmidecode") {
		t.Errorf("dmidecode output missing header")
	}

	// Verify metadata file
	metaPath := filepath.Join(metaDataDir, "insights.specs.Specs.dmidecode.json")
	if _, err := os.Stat(metaPath); os.IsNotExist(err) {
		t.Errorf("metadata file not created")
	}
}

func TestWriteMACAddresses(t *testing.T) {
	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "data")
	metaDataDir := filepath.Join(tmpDir, "meta_data")

	if err := os.MkdirAll(dataDir, 0700); err != nil {
		t.Fatalf("failed to create data dir: %v", err)
	}
	if err := os.MkdirAll(metaDataDir, 0700); err != nil {
		t.Fatalf("failed to create meta_data dir: %v", err)
	}

	err := writeMACAddresses(dataDir, metaDataDir)
	if err != nil {
		t.Fatalf("writeMACAddresses failed: %v", err)
	}

	// Verify metadata file exists
	metaPath := filepath.Join(metaDataDir, "insights.specs.Specs.mac_addresses.json")
	content, err := os.ReadFile(metaPath)
	if err != nil {
		t.Fatalf("failed to read mac_addresses metadata: %v", err)
	}

	// Parse and verify it's valid JSON with array results
	var metadata MetadataSpec
	if err := json.Unmarshal(content, &metadata); err != nil {
		t.Fatalf("failed to parse metadata JSON: %v", err)
	}

	if metadata.Name != "insights.specs.Specs.mac_addresses" {
		t.Errorf("expected name=insights.specs.Specs.mac_addresses, got %s", metadata.Name)
	}

	// Results should be an array
	results, ok := metadata.Results.([]any)
	if !ok {
		t.Fatalf("expected Results to be array, got %T", metadata.Results)
	}

	// Should have at least one interface (loopback at minimum)
	if len(results) == 0 {
		t.Errorf("expected at least one interface, got 0")
	}

	// Verify data files were created
	sysNetDir := filepath.Join(dataDir, "sys", "class", "net")
	if _, err := os.Stat(sysNetDir); os.IsNotExist(err) {
		t.Errorf("sys/class/net directory not created")
	}
}
