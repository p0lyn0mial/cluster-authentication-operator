package jsonpatcher

import (
	"encoding/json"
	"testing"

	"github.com/google/go-cmp/cmp"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/runtime"

	jsonpatch "gopkg.in/evanphx/json-patch.v4"
	"sigs.k8s.io/yaml"
)

func TestGenerateJSONPatch(t *testing.T) {
	tests := []struct {
		name        string
		inputYAML   string
		expectedYML string
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
			expectedYML: `
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
			expectedYML: `
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
			runPatchGenerationTest(t, tt.inputYAML, tt.expectedYML)
		})
	}
}

func TestGenerateJSONPatchAnnotations(t *testing.T) {
	tests := []struct {
		name        string
		inputYAML   string
		expectedYML string
	}{
		{
			name: "adds annotations to empty metadata",
			inputYAML: `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: demo
  namespace: default
spec:
  replicas: 1
  template:
    metadata:
      labels:
        app: demo
    spec: {}
`,
			expectedYML: `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: demo
  namespace: default
  annotations:
    foo: bar
    baz: qux
spec:
  replicas: 1
  template:
    metadata:
      labels:
        app: demo
    spec: {}
`,
		},
		{
			name: "appends to existing annotations map",
			inputYAML: `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: demo
  namespace: default
  annotations:
    existing: keep
spec:
  replicas: 1
  template:
    metadata:
      labels:
        app: demo
    spec: {}
`,
			expectedYML: `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: demo
  namespace: default
  annotations:
    existing: keep
    new-anno: new
spec:
  replicas: 1
  template:
    metadata:
      labels:
        app: demo
    spec: {}
`,
		},
		{
			name: "add operation overwrites an existing annotation",
			inputYAML: `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: demo
  namespace: default
  annotations:
    env: stage
spec:
  replicas: 1
  template:
    metadata:
      labels:
        app: demo
    spec: {}
`,
			expectedYML: `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: demo
  namespace: default
  annotations:
    env: prod
spec:
  replicas: 1
  template:
    metadata:
      labels:
        app: demo
    spec: {}
`,
		},
		{
			name: "add operation with identical value keeps single annotation",
			inputYAML: `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: demo
  namespace: default
  annotations:
    env: prod
spec:
  replicas: 1
  template:
    metadata:
      labels:
        app: demo
    spec: {}
`,
			expectedYML: `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: demo
  namespace: default
  annotations:
    env: prod
spec:
  replicas: 1
  template:
    metadata:
      labels:
        app: demo
    spec: {}
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runPatchGenerationTest(t, tt.inputYAML, tt.expectedYML)
		})
	}
}

func TestGenerateJSONPatchLabels(t *testing.T) {
	tests := []struct {
		name        string
		inputYAML   string
		expectedYML string
	}{
		{
			name: "adds labels to empty metadata",
			inputYAML: `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: demo
  namespace: default
spec:
  replicas: 1
  template:
    metadata: {}
    spec: {}
`,
			expectedYML: `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: demo
  namespace: default
  labels:
    foo: bar
    baz: qux
spec:
  replicas: 1
  template:
    metadata: {}
    spec: {}
`,
		},
		{
			name: "appends to existing labels map",
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
			expectedYML: `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: demo
  namespace: default
  labels:
    app: demo
    env: test
spec:
  replicas: 1
  template:
    metadata:
      labels:
        app: demo
    spec: {}
`,
		},
		{
			name: "add operation overwrites existing label",
			inputYAML: `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: demo
  namespace: default
  labels:
    app: demo
    env: stage
spec:
  replicas: 1
  template:
    metadata:
      labels:
        app: demo
    spec: {}
`,
			expectedYML: `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: demo
  namespace: default
  labels:
    app: demo
    env: prod
spec:
  replicas: 1
  template:
    metadata:
      labels:
        app: demo
    spec: {}
`,
		},
		{
			name: "add operation with identical label keeps single entry",
			inputYAML: `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: demo
  namespace: default
  labels:
    app: demo
    env: prod
spec:
  replicas: 1
  template:
    metadata:
      labels:
        app: demo
    spec: {}
`,
			expectedYML: `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: demo
  namespace: default
  labels:
    app: demo
    env: prod
spec:
  replicas: 1
  template:
    metadata:
      labels:
        app: demo
    spec: {}
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runPatchGenerationTest(t, tt.inputYAML, tt.expectedYML)
		})
	}
}

func TestGenerateJSONPatchPriorityClass(t *testing.T) {
	tests := []struct {
		name        string
		inputYAML   string
		expectedYML string
	}{
		{
			name: "adds priority class when unset",
			inputYAML: `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: demo
  namespace: default
spec:
  replicas: 1
  template:
    metadata:
      labels:
        app: demo
    spec: {}
`,
			expectedYML: `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: demo
  namespace: default
spec:
  replicas: 1
  template:
    metadata:
      labels:
        app: demo
    spec:
      priorityClassName: high-priority
`,
		},
		{
			name: "replaces existing priority class",
			inputYAML: `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: demo
  namespace: default
spec:
  replicas: 1
  template:
    metadata:
      labels:
        app: demo
    spec:
      priorityClassName: medium
`,
			expectedYML: `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: demo
  namespace: default
spec:
  replicas: 1
  template:
    metadata:
      labels:
        app: demo
    spec:
      priorityClassName: critical
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runPatchGenerationTest(t, tt.inputYAML, tt.expectedYML)
		})
	}
}

func TestGenerateJSONPatchNodeSelector(t *testing.T) {
	tests := []struct {
		name        string
		inputYAML   string
		expectedYML string
	}{
		{
			name: "adds node selector when empty",
			inputYAML: `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: demo
  namespace: default
spec:
  replicas: 1
  template:
    metadata:
      labels:
        app: demo
    spec: {}
`,
			expectedYML: `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: demo
  namespace: default
spec:
  replicas: 1
  template:
    metadata:
      labels:
        app: demo
    spec:
      nodeSelector:
        role: worker
        zone: east
`,
		},
		{
			name: "appends selector entry",
			inputYAML: `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: demo
  namespace: default
spec:
  replicas: 1
  template:
    metadata:
      labels:
        app: demo
    spec:
      nodeSelector:
        role: worker
`,
			expectedYML: `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: demo
  namespace: default
spec:
  replicas: 1
  template:
    metadata:
      labels:
        app: demo
    spec:
      nodeSelector:
        role: worker
        zone: west
`,
		},
		{
			name: "overwrites existing selector key",
			inputYAML: `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: demo
  namespace: default
spec:
  replicas: 1
  template:
    metadata:
      labels:
        app: demo
    spec:
      nodeSelector:
        role: worker
        zone: east
`,
			expectedYML: `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: demo
  namespace: default
spec:
  replicas: 1
  template:
    metadata:
      labels:
        app: demo
    spec:
      nodeSelector:
        role: worker
        zone: central
`,
		},
		{
			name: "add keeps single entry when identical",
			inputYAML: `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: demo
  namespace: default
spec:
  replicas: 1
  template:
    metadata:
      labels:
        app: demo
    spec:
      nodeSelector:
        role: worker
        zone: central
`,
			expectedYML: `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: demo
  namespace: default
spec:
  replicas: 1
  template:
    metadata:
      labels:
        app: demo
    spec:
      nodeSelector:
        role: worker
        zone: central
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runPatchGenerationTest(t, tt.inputYAML, tt.expectedYML)
		})
	}
}

func TestGenerateJSONPatchTolerations(t *testing.T) {
	tests := []struct {
		name        string
		inputYAML   string
		expectedYML string
	}{
		{
			name: "identical tolerations produce empty patch",
			inputYAML: `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: demo
  namespace: default
spec:
  replicas: 1
  template:
    metadata:
      labels:
        app: demo
    spec:
      tolerations:
      - key: dedicated
        operator: Equal
        value: infra
        effect: NoSchedule
`,
			expectedYML: `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: demo
  namespace: default
spec:
  replicas: 1
  template:
    metadata:
      labels:
        app: demo
    spec:
      tolerations:
      - key: dedicated
        operator: Equal
        value: infra
        effect: NoSchedule
`,
		},
		{
			name: "adds first toleration entry",
			inputYAML: `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: demo
  namespace: default
spec:
  replicas: 1
  template:
    metadata:
      labels:
        app: demo
    spec: {}
`,
			expectedYML: `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: demo
  namespace: default
spec:
  replicas: 1
  template:
    metadata:
      labels:
        app: demo
    spec:
      tolerations:
      - key: dedicated
        operator: Equal
        value: infra
        effect: NoSchedule
`,
		},
		{
			name: "appends additional toleration",
			inputYAML: `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: demo
  namespace: default
spec:
  replicas: 1
  template:
    metadata:
      labels:
        app: demo
    spec:
      tolerations:
      - key: dedicated
        operator: Equal
        value: infra
        effect: NoSchedule
`,
			expectedYML: `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: demo
  namespace: default
spec:
  replicas: 1
  template:
    metadata:
      labels:
        app: demo
    spec:
      tolerations:
      - key: dedicated
        operator: Equal
        value: infra
        effect: NoSchedule
      - key: workload
        operator: Exists
        effect: NoExecute
`,
		},
		{
			name: "updates existing toleration values",
			inputYAML: `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: demo
  namespace: default
spec:
  replicas: 1
  template:
    metadata:
      labels:
        app: demo
    spec:
      tolerations:
      - key: workload
        operator: Equal
        value: batch
        effect: NoExecute
        tolerationSeconds: 120
`,
			expectedYML: `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: demo
  namespace: default
spec:
  replicas: 1
  template:
    metadata:
      labels:
        app: demo
    spec:
      tolerations:
      - key: workload
        operator: Equal
        value: critical
        effect: NoSchedule
        tolerationSeconds: 0
`,
		},
		{
			name: "removes specific toleration entry",
			inputYAML: `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: demo
  namespace: default
spec:
  replicas: 1
  template:
    metadata:
      labels:
        app: demo
    spec:
      tolerations:
      - key: keep
        operator: Exists
        effect: NoSchedule
      - key: remove
        operator: Equal
        value: worker
        effect: PreferNoSchedule
`,
			expectedYML: `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: demo
  namespace: default
spec:
  replicas: 1
  template:
    metadata:
      labels:
        app: demo
    spec:
      tolerations:
      - key: keep
        operator: Exists
        effect: NoSchedule
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runPatchGenerationTest(t, tt.inputYAML, tt.expectedYML)
		})
	}
}

func TestGenerateJSONPatchErrors(t *testing.T) {
	dep := mustDeploymentFromYAML(t, `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: demo
  namespace: default
`)
	if _, err := GenerateJSONPatch(nil, dep); err == nil {
		t.Fatalf("expected error when original is nil")
	}
	if _, err := GenerateJSONPatch(dep, nil); err == nil {
		t.Fatalf("expected error when modified is nil")
	}
}

func runPatchGenerationTest(t *testing.T, inputYAML, expectedYAML string) {
	t.Helper()

	target := mustDeploymentFromYAML(t, inputYAML)
	expected := mustDeploymentFromYAML(t, expectedYAML)

	patchBytes, err := GenerateJSONPatch(target.DeepCopyObject(), expected.DeepCopyObject())
	if err != nil {
		t.Fatalf("failed to generate jsonpatch: %v", err)
	}

	patched := target.DeepCopyObject()
	applyPatchBytes(t, patched, patchBytes)

	patchedDep, ok := patched.(*appsv1.Deployment)
	if !ok {
		t.Fatalf("unexpected patched type %T", patched)
	}

	if diff := cmp.Diff(expected, patchedDep); diff != "" {
		t.Fatalf("unexpected diff (-want +got):\n%s", diff)
	}
}

func applyPatchBytes(t *testing.T, obj runtime.Object, patchBytes []byte) {
	t.Helper()

	patch, err := jsonpatch.DecodePatch(patchBytes)
	if err != nil {
		t.Fatalf("failed to decode patch: %v", err)
	}

	original, err := json.Marshal(obj)
	if err != nil {
		t.Fatalf("failed to marshal runtime object: %v", err)
	}

	patched, err := patch.Apply(original)
	if err != nil {
		t.Fatalf("failed to apply patch: %v", err)
	}

	if err := json.Unmarshal(patched, obj); err != nil {
		t.Fatalf("failed to unmarshal patched object: %v", err)
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
