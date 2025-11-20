package patchalchemy

import (
	"encoding/json"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"

	jsonpatch "gopkg.in/evanphx/json-patch.v4"
	"sigs.k8s.io/yaml"
)

// DefaultPatchKey declares the ConfigMap data key that stores the patch document.
const DefaultPatchKey = "patch.yaml"

// ApplyConfigMapPatch loads a jsonpatch definition from the provided ConfigMap and applies it to the runtime object.
func ApplyConfigMapPatch(obj runtime.Object, cm *corev1.ConfigMap) error {
	if obj == nil {
		return fmt.Errorf("target runtime object is nil")
	}
	if cm == nil {
		return fmt.Errorf("configmap is nil")
	}

	rawPatch, ok := cm.Data[DefaultPatchKey]
	if !ok {
		return fmt.Errorf("configmap %s/%s does not include %q", cm.Namespace, cm.Name, DefaultPatchKey)
	}

	patchBytes, err := extractPatchBytes([]byte(rawPatch))
	if err != nil {
		return fmt.Errorf("failed to interpret patch document: %w", err)
	}

	patch, err := jsonpatch.DecodePatch(patchBytes)
	if err != nil {
		return fmt.Errorf("invalid jsonpatch: %w", err)
	}

	original, err := json.Marshal(obj)
	if err != nil {
		return fmt.Errorf("failed to marshal object for patching: %w", err)
	}

	patched, err := patch.Apply(original)
	if err != nil {
		return fmt.Errorf("failed to apply patch: %w", err)
	}

	if err := json.Unmarshal(patched, obj); err != nil {
		return fmt.Errorf("failed to unmarshal patched object: %w", err)
	}

	return nil
}

type jsonPatchDocument struct {
	Spec struct {
		Patch []map[string]any `json:"patch" yaml:"patch"`
	} `json:"spec" yaml:"spec"`
}

func extractPatchBytes(document []byte) ([]byte, error) {
	var parsed jsonPatchDocument
	if err := yaml.Unmarshal(document, &parsed); err != nil {
		return nil, err
	}
	if len(parsed.Spec.Patch) == 0 {
		return nil, fmt.Errorf("spec.patch is empty")
	}
	return json.Marshal(parsed.Spec.Patch)
}
