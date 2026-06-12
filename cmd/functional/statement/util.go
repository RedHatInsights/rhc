package statement

import (
	"fmt"
	"os"
	"strings"
)

// jsonLookup traverses a decoded JSON document using a dot-separated path such
// as ".connected" or ".features.content.preference".
func jsonLookup(data interface{}, field string) (interface{}, error) {
	field = strings.TrimPrefix(field, ".")
	parts := strings.Split(field, ".")
	current := data
	for _, part := range parts {
		m, ok := current.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("expected object at %q, got %T", part, current)
		}
		val, exists := m[part]
		if !exists {
			return nil, fmt.Errorf("key %q not found", part)
		}
		current = val
	}
	return current, nil
}

// consumerCertExists returns an error when /etc/pki/consumer/cert.pem is absent.
func consumerCertExists() error {
	const consumerCert = "/etc/pki/consumer/cert.pem"
	if _, err := os.Stat(consumerCert); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf(
				"system is not registered: %s does not exist",
				consumerCert,
			)
		}
		return fmt.Errorf("stat %s: %w", consumerCert, err)
	}
	return nil
}
