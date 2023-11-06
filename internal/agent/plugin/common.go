package plugin

import (
	"encoding/json"
	"fmt"

	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/log"
	"gopkg.in/yaml.v1"
)

func UnmarshalRawJSONToYaml(input []byte) ([]byte, error) {
	yamlData := []byte{}
	if len(input) == 0 {
		log.Debug("nothing to decode")
		return yamlData, nil
	}

	var jsonData any
	if err := json.Unmarshal(input, &jsonData); err != nil {
		return nil, fmt.Errorf("unmarshalling raw json: %w", err)
	}

	yamlData, err := yaml.Marshal(jsonData)
	if err != nil {
		return nil, fmt.Errorf("marshalling raw json to yaml: %w", err)
	}

	return yamlData, nil
}
