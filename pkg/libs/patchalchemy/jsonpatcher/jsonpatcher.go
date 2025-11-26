package jsonpatcher

import (
	"encoding/json"
	"fmt"

	jsonpatch "github.com/mattbaird/jsonpatch"
	"k8s.io/apimachinery/pkg/runtime"
)

// GenerateJSONPatch produces an RFC6902 patch document describing how to turn original into modified.
// The returned byte slice is a JSON array that can be fed into any JSON patch implementation.
func GenerateJSONPatch(original, modified runtime.Object) ([]byte, error) {
	if original == nil {
		return nil, fmt.Errorf("original object is nil")
	}
	if modified == nil {
		return nil, fmt.Errorf("modified object is nil")
	}

	originalBytes, err := json.Marshal(original)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal original object: %w", err)
	}
	modifiedBytes, err := json.Marshal(modified)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal modified object: %w", err)
	}

	patch, err := jsonpatch.CreatePatch(originalBytes, modifiedBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to compute jsonpatch: %w", err)
	}

	if len(patch) == 0 {
		return []byte("[]"), nil
	}

	patchBytes, err := json.Marshal(patch)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal jsonpatch: %w", err)
	}
	return patchBytes, nil
}
