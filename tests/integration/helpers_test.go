package integration_test

import "encoding/json"

// testConvertToRawMessage converts a map[string]interface{} to map[string]json.RawMessage for testing
func testConvertToRawMessage(input map[string]interface{}) map[string]json.RawMessage {
	result := make(map[string]json.RawMessage)
	for k, v := range input {
		data, err := json.Marshal(v)
		if err != nil {
			panic(err) // In tests, we can panic on marshal errors
		}
		result[k] = json.RawMessage(data)
	}
	return result
}
