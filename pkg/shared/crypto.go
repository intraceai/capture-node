package shared

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
)

func SHA256Hex(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

func CanonicalJSON(v interface{}) ([]byte, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}

	var intermediate interface{}
	if err := json.Unmarshal(data, &intermediate); err != nil {
		return nil, err
	}

	return marshalCanonical(intermediate)
}

func marshalCanonical(v interface{}) ([]byte, error) {
	switch val := v.(type) {
	case map[string]interface{}:
		keys := make([]string, 0, len(val))
		for k := range val {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		result := []byte("{")
		for i, k := range keys {
			if i > 0 {
				result = append(result, ',')
			}
			keyBytes, _ := json.Marshal(k)
			result = append(result, keyBytes...)
			result = append(result, ':')
			valBytes, err := marshalCanonical(val[k])
			if err != nil {
				return nil, err
			}
			result = append(result, valBytes...)
		}
		result = append(result, '}')
		return result, nil

	case []interface{}:
		result := []byte("[")
		for i, item := range val {
			if i > 0 {
				result = append(result, ',')
			}
			itemBytes, err := marshalCanonical(item)
			if err != nil {
				return nil, err
			}
			result = append(result, itemBytes...)
		}
		result = append(result, ']')
		return result, nil

	default:
		return json.Marshal(v)
	}
}
