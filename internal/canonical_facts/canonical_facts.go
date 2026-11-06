package canonical_facts

import (
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/google/uuid"
)

// An InvalidValueTypeError represents an error when serializing data into an
// unsupported destination.
type InvalidValueTypeError struct {
	key string
	val interface{}
}

func (e InvalidValueTypeError) Error() string {
	return fmt.Sprintf("invalid type '%T' for key '%s'", e.val, e.key)
}

// CanonicalFacts contain several identification strings that collectively
// combine to uniquely identify a system to the platform services.
type CanonicalFacts struct {
	InsightsID            string   `json:"insights_id"`
	MachineID             string   `json:"machine_id"`
	BIOSUUID              string   `json:"bios_uuid"`
	SubscriptionManagerID string   `json:"subscription_manager_id"`
	IPAddresses           []string `json:"ip_addresses"`
	MACAddresses          []string `json:"mac_addresses"`
	FQDN                  string   `json:"fqdn"`
}

// CanonicalFactsFromMap creates a CanonicalFacts struct from the key-value
// pairs in a map.
func CanonicalFactsFromMap(m map[string]interface{}) (*CanonicalFacts, error) {
	var facts CanonicalFacts

	if val, ok := m["insights_id"]; ok {
		switch val := val.(type) {
		case string:
			facts.InsightsID = val
		default:
			return nil, &InvalidValueTypeError{key: "insights_id", val: val}
		}
	}

	if val, ok := m["machine_id"]; ok {
		switch val := val.(type) {
		case string:
			facts.MachineID = val
		default:
			return nil, &InvalidValueTypeError{key: "machine_id", val: val}
		}
	}

	if val, ok := m["bios_uuid"]; ok {
		switch val := val.(type) {
		case string:
			facts.BIOSUUID = val
		default:
			return nil, &InvalidValueTypeError{key: "bios_uuid", val: val}
		}
	}

	if val, ok := m["subscription_manager_id"]; ok {
		switch val := val.(type) {
		case string:
			facts.SubscriptionManagerID = val
		default:
			return nil, &InvalidValueTypeError{key: "subscription_manager_id", val: val}
		}
	}

	if val, ok := m["ip_addresses"]; ok {
		switch val := val.(type) {
		case []string:
			facts.IPAddresses = val
		default:
			return nil, &InvalidValueTypeError{key: "ip_addresses", val: val}
		}
	}

	if val, ok := m["fqdn"]; ok {
		switch val := val.(type) {
		case string:
			facts.FQDN = val
		default:
			return nil, &InvalidValueTypeError{key: "fqdn", val: val}
		}
	}

	if val, ok := m["mac_addresses"]; ok {
		switch val := val.(type) {
		case []string:
			facts.MACAddresses = val
		default:
			return nil, &InvalidValueTypeError{key: "mac_addresses", val: val}
		}
	}

	return &facts, nil
}

// factSources holds filesystem paths used when collecting canonical facts.
type factSources struct {
	insightsIDPath string
	machineIDPath  string
	biosUUIDPath   string
	consumerCert   string
}

var defaultFactSources = factSources{
	insightsIDPath: "/etc/insights-client/machine-id",
	machineIDPath:  "/etc/machine-id",
	biosUUIDPath:   "/sys/devices/virtual/dmi/id/product_uuid",
	consumerCert:   "/etc/pki/consumer/cert.pem",
}

// GetCanonicalFacts attempts to construct a CanonicalFacts struct by collecting
// data from the localhost. Collection is best-effort: if an individual fact
// cannot be read, that field is left empty and remaining facts are still
// collected (CCT-2684).
func GetCanonicalFacts() (*CanonicalFacts, error) {
	return getCanonicalFacts(defaultFactSources)
}

// getCanonicalFacts collects canonical facts using the provided source paths.
// Errors from individual collectors are ignored so partial results are returned.
func getCanonicalFacts(sources factSources) (*CanonicalFacts, error) {
	facts := &CanonicalFacts{
		IPAddresses:  []string{},
		MACAddresses: []string{},
	}

	if insightsID, err := readFile(sources.insightsIDPath); err == nil {
		facts.InsightsID = insightsID
	}

	if machineID, err := readFile(sources.machineIDPath); err == nil {
		if id, err := toUUIDv4(machineID); err == nil {
			facts.MachineID = id
		}
	}

	if biosUUID, err := readFile(sources.biosUUIDPath); err == nil {
		facts.BIOSUUID = biosUUID
	}

	if subscriptionManagerID, err := readCert(sources.consumerCert); err == nil {
		facts.SubscriptionManagerID = subscriptionManagerID
	}

	if ipAddresses, err := collectIPAddresses(); err == nil {
		facts.IPAddresses = ipAddresses
	}

	if fqdn, err := os.Hostname(); err == nil {
		facts.FQDN = fqdn
	}

	if macAddresses, err := collectMACAddresses(); err == nil {
		facts.MACAddresses = macAddresses
	}

	return facts, nil
}

// readFile reads the contents of filename into a string, trims whitespace,
// and returns the result.
func readFile(filename string) (string, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

// readCert reads the data in filename, decodes it if necessary, and returns
// the certificate subject CN.
func readCert(filename string) (string, error) {
	var asn1Data []byte
	switch filepath.Ext(filename) {
	case ".pem":
		data, err := os.ReadFile(filename)
		if err != nil {
			return "", err
		}

		block, _ := pem.Decode(data)
		if block == nil {
			return "", fmt.Errorf("failed to decode PEM data: %v", filename)
		}
		asn1Data = append(asn1Data, block.Bytes...)
	default:
		var err error
		asn1Data, err = os.ReadFile(filename)
		if err != nil {
			return "", err
		}
	}

	cert, err := x509.ParseCertificate(asn1Data)
	if err != nil {
		return "", err
	}
	return cert.Subject.CommonName, nil
}

// collectIPAddresses iterates over network interfaces and collects IP
// addresses.
func collectIPAddresses() ([]string, error) {
	addresses := make([]string, 0)
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagLoopback == net.FlagLoopback {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			return nil, err
		}
		for _, addr := range addrs {
			switch addr := addr.(type) {
			case *net.IPNet:
				netAddr := addr
				if netAddr.IP.To4() == nil {
					continue
				}
				addresses = append(addresses, netAddr.IP.String())
			}
		}
	}

	return addresses, nil
}

// collectMACAddresses iterates over network interfaces and collects hardware
// addresses.
func collectMACAddresses() ([]string, error) {
	addresses := make([]string, 0)
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	sort.Slice(ifaces, func(i, j int) bool {
		return ifaces[i].Name < ifaces[j].Name
	})
	for _, iface := range ifaces {
		addr := iface.HardwareAddr.String()
		if addr == "" {
			addr = "00:00:00:00:00:00"
		}
		addresses = append(addresses, addr)
	}
	return addresses, nil
}

// toUUIDv4 parses id as a UUID and returns the "dashed" notation string format.
func toUUIDv4(id string) (string, error) {
	UUID, err := uuid.Parse(id)
	if err != nil {
		return "", err
	}
	return UUID.String(), nil
}
