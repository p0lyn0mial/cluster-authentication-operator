package patchalchemy

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	"sigs.k8s.io/yaml"
)

func TestApplyConfigMapPatch(t *testing.T) {
	tests := []struct {
		name         string
		inputYAML    string
		configMapYML string
		expectedYAML string
	}{
		{
			name: "updates replicas and adds label",
			inputYAML: `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: demo
  namespace: default
  labels:
    app: demo
spec:
  replicas: 1
  template:
    metadata:
      labels:
        app: demo
    spec: {}
`,
			configMapYML: `
apiVersion: v1
kind: ConfigMap
metadata:
  name: jsonpatch-config
  namespace: default
data:
  patch.yaml: |-
    apiVersion: jsonpatch.openshift.io/v1alpha1
    kind: JsonPatch
    metadata:
      name: demo
    spec:
      patch:
      - op: replace
        path: /spec/replicas
        value: 3
      - op: add
        path: /metadata/labels/env
        value: test
`,
			expectedYAML: `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: demo
  namespace: default
  labels:
    app: demo
    env: test
spec:
  replicas: 3
  template:
    metadata:
      labels:
        app: demo
    spec: {}
`,
		},
		{
			name: "applies pod affinity override",
			inputYAML: `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: demo
  namespace: default
  labels:
    app: demo
spec:
  replicas: 1
  template:
    metadata:
      labels:
        app: demo
    spec:
      affinity:
        podAffinity:
          preferredDuringSchedulingIgnoredDuringExecution: []
`,
			configMapYML: `
apiVersion: v1
kind: ConfigMap
metadata:
  name: jsonpatch-config
  namespace: default
data:
  patch.yaml: |-
    apiVersion: jsonpatch.openshift.io/v1alpha1
    kind: JsonPatch
    metadata:
      name: affinity
    spec:
      patch:
      - op: replace
        path: /spec/template/spec/affinity/podAffinity
        value:
          requiredDuringSchedulingIgnoredDuringExecution:
          - labelSelector:
              matchLabels:
                app: backend
            topologyKey: kubernetes.io/hostname
`,
			expectedYAML: `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: demo
  namespace: default
  labels:
    app: demo
spec:
  replicas: 1
  template:
    metadata:
      labels:
        app: demo
    spec:
      affinity:
        podAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
          - labelSelector:
              matchLabels:
                app: backend
            topologyKey: kubernetes.io/hostname
          preferredDuringSchedulingIgnoredDuringExecution: []
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target := mustDeploymentFromYAML(t, tt.inputYAML)
			targetConfigMap := mustConfigMapFromYAML(t, tt.configMapYML)

			if err := ApplyConfigMapPatch(target, targetConfigMap); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			expected := mustDeploymentFromYAML(t, tt.expectedYAML)
			if diff := cmp.Diff(expected, target); diff != "" {
				t.Fatalf("unexpected diff (-want +got):\n%s", diff)
			}
		})
	}
}

func mustDeploymentFromYAML(t *testing.T, doc string) *appsv1.Deployment {
	t.Helper()
	var dep appsv1.Deployment
	if err := yaml.Unmarshal([]byte(doc), &dep); err != nil {
		t.Fatalf("failed to load deployment from yaml: %v", err)
	}
	return &dep
}

func mustConfigMapFromYAML(t *testing.T, doc string) *corev1.ConfigMap {
	t.Helper()
	var cm corev1.ConfigMap
	if err := yaml.Unmarshal([]byte(doc), &cm); err != nil {
		t.Fatalf("failed to load configmap from yaml: %v", err)
	}
	return &cm
}
