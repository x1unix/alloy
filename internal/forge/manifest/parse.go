package manifest

import (
	"fmt"
	"io"

	"gopkg.in/yaml.v3"
)

// Parse decodes a forge manifest from the given reader.
// It does not validate the manifest; call Validate separately.
func Parse(r io.Reader) (*Manifest, error) {
	var m Manifest
	dec := yaml.NewDecoder(r)
	dec.KnownFields(true)
	if err := dec.Decode(&m); err != nil {
		return nil, fmt.Errorf("decode manifest: %w", err)
	}
	return &m, nil
}
