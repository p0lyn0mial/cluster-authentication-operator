// Code generated by go-bindata.
// sources:
// bindata/oauth-apiserver/apiserver-clusterrolebinding.yaml
// bindata/oauth-apiserver/cm.yaml
// bindata/oauth-apiserver/ds.yaml
// bindata/oauth-apiserver/ns.yaml
// bindata/oauth-apiserver/sa.yaml
// bindata/oauth-apiserver/svc.yaml
// bindata/oauth-openshift/deployment.yaml
// DO NOT EDIT!

package assets

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type asset struct {
	bytes []byte
	info  os.FileInfo
}

type bindataFileInfo struct {
	name    string
	size    int64
	mode    os.FileMode
	modTime time.Time
}

func (fi bindataFileInfo) Name() string {
	return fi.name
}
func (fi bindataFileInfo) Size() int64 {
	return fi.size
}
func (fi bindataFileInfo) Mode() os.FileMode {
	return fi.mode
}
func (fi bindataFileInfo) ModTime() time.Time {
	return fi.modTime
}
func (fi bindataFileInfo) IsDir() bool {
	return false
}
func (fi bindataFileInfo) Sys() interface{} {
	return nil
}

var _oauthApiserverApiserverClusterrolebindingYaml = []byte(`apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: system:openshift:oauth-apiserver
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: cluster-admin
subjects:
- kind: ServiceAccount
  namespace: openshift-oauth-apiserver
  name: oauth-apiserver-sa`)

func oauthApiserverApiserverClusterrolebindingYamlBytes() ([]byte, error) {
	return _oauthApiserverApiserverClusterrolebindingYaml, nil
}

func oauthApiserverApiserverClusterrolebindingYaml() (*asset, error) {
	bytes, err := oauthApiserverApiserverClusterrolebindingYamlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "oauth-apiserver/apiserver-clusterrolebinding.yaml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _oauthApiserverCmYaml = []byte(`apiVersion: v1
kind: ConfigMap
metadata:
  namespace: openshift-oauth-apiserver
  name: config
data:
  audit-policy.yaml: |
    apiVersion: audit.k8s.io/v1beta1
    kind: Policy
    # Don't generate audit events for all requests in RequestReceived stage.
    omitStages:
    - "RequestReceived"
    rules:
    # Don't log requests for events
    - level: None
      resources:
      - group: ""
        resources: ["events"]
    # Don't log oauth tokens as metadata.name is the secret
    - level: None
      resources:
      - group: "oauth.openshift.io"
        resources: ["oauthaccesstokens", "oauthauthorizetokens"]
    # Don't log authenticated requests to certain non-resource URL paths.
    - level: None
      userGroups: ["system:authenticated", "system:unauthenticated"]
      nonResourceURLs:
      - "/api*" # Wildcard matching.
      - "/version"
      - "/healthz"
    # Log the full Identity API resource object so that the audit trail
    # allows us to match the username with the IDP identity.
    - level: RequestResponse
      verbs: ["create", "update", "patch"]
      resources:
      - group: "user.openshift.io"
        resources: ["identities"]
    # A catch-all rule to log all other requests at the Metadata level.
    - level: Metadata
      # Long-running requests like watches that fall under this rule will not
      # generate an audit event in RequestReceived.
      omitStages:
      - "RequestReceived"
`)

func oauthApiserverCmYamlBytes() ([]byte, error) {
	return _oauthApiserverCmYaml, nil
}

func oauthApiserverCmYaml() (*asset, error) {
	bytes, err := oauthApiserverCmYamlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "oauth-apiserver/cm.yaml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _oauthApiserverDsYaml = []byte(`apiVersion: apps/v1
kind: DaemonSet
metadata:
  namespace: openshift-oauth-apiserver
  name: apiserver
  labels:
    app: openshift-oauth-apiserver
    apiserver: "true"
spec:
  updateStrategy:
    type: RollingUpdate
  selector:
    matchLabels:
      app: openshift-oauth-apiserver
      apiserver: "true"
  template:
    metadata:
      name: openshift-oauth-apiserver
      labels:
        app: openshift-oauth-apiserver
        apiserver: "true"
    spec:
      serviceAccountName: oauth-apiserver-sa
      priorityClassName: system-node-critical
      initContainers:
        - name: fix-audit-permissions
          terminationMessagePolicy: FallbackToLogsOnError
          image: ${IMAGE}
          imagePullPolicy: IfNotPresent
          command: ['sh', '-c', 'chmod 0700 /var/log/oauth-apiserver']
          securityContext:
            privileged: true
          volumeMounts:
            - mountPath: /var/log/oauth-apiserver
              name: audit-dir
      containers:
      - name: oauth-apiserver
        terminationMessagePolicy: FallbackToLogsOnError
        image: ${IMAGE}
        imagePullPolicy: IfNotPresent
        command: ["/bin/bash", "-ec"]
        args:
          - |
            if [ -s /var/run/configmaps/trusted-ca-bundle/tls-ca-bundle.pem ]; then
              echo "Copying system trust bundle"
              cp -f /var/run/configmaps/trusted-ca-bundle/tls-ca-bundle.pem /etc/pki/ca-trust/extracted/pem/tls-ca-bundle.pem
            fi
            exec oauth-apiserver start \
              --secure-port=8443 \
              --audit-log-path=/var/log/oauth-apiserver/audit.log \
              --audit-log-format=json \
              --audit-log-maxsize=100 \
              --audit-log-maxbackup=10 \
              --audit-policy-file=/var/run/configmaps/config/audit-policy.yaml \
              --etcd-servers=https://etcd.openshift-etcd.svc:2379 \
              --etcd-cafile=/var/run/configmaps/etcd-serving-ca/ca-bundle.crt \
              --etcd-keyfile=/var/run/secrets/etcd-client/tls.key \
              --etcd-certfile=/var/run/secrets/etcd-client/tls.crt \
              --shutdown-delay-duration=3s \
              --v=2
          # TODO: enable encryption support
          # --encryption-provider-config=/var/run/secrets/encryption-config
        resources:
          requests:
            memory: 200Mi
            cpu: 150m
        # we need to set this to privileged to be able to write audit to /var/log/oauth-apiserver
        securityContext:
          privileged: true
        ports:
        - containerPort: 8443
        volumeMounts:
        - mountPath: /var/run/configmaps/config
          name: config
        - mountPath: /var/run/secrets/etcd-client
          name: etcd-client
        - mountPath: /var/run/configmaps/etcd-serving-ca
          name: etcd-serving-ca
        - mountPath: /var/run/configmaps/trusted-ca-bundle
          name: trusted-ca-bundle
        - mountPath: /var/run/secrets/serving-cert
          name: serving-cert
        - mountPath: /var/run/secrets/encryption-config
          name: encryption-config
        - mountPath: /var/log/oauth-apiserver
          name: audit-dir
        livenessProbe:
          initialDelaySeconds: 30
          httpGet:
            scheme: HTTPS
            port: 8443
            path: healthz
        readinessProbe:
          failureThreshold: 10
          httpGet:
            scheme: HTTPS
            port: 8443
            path: healthz
      terminationGracePeriodSeconds: 70 # a bit more than the 60 seconds timeout of non-long-running requests
      volumes:
      - name: config
        configMap:
          name: config
      - name: etcd-client
        secret:
          secretName: etcd-client
      - name: etcd-serving-ca
        configMap:
          name: etcd-serving-ca
      - name: serving-cert
        secret:
          secretName: serving-cert
      - name: trusted-ca-bundle
        configMap:
          name: trusted-ca-bundle
          optional: true
          items:
          - key: ca-bundle.crt
            path: tls-ca-bundle.pem
      - name: encryption-config
        secret:
          secretName: encryption-config-${REVISION}
          optional: true
      - hostPath:
          path: /var/log/oauth-apiserver
        name: audit-dir
      nodeSelector:
        node-role.kubernetes.io/master: ""
      tolerations:
      - operator: Exists
`)

func oauthApiserverDsYamlBytes() ([]byte, error) {
	return _oauthApiserverDsYaml, nil
}

func oauthApiserverDsYaml() (*asset, error) {
	bytes, err := oauthApiserverDsYamlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "oauth-apiserver/ds.yaml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _oauthApiserverNsYaml = []byte(`apiVersion: v1
kind: Namespace
metadata:
  annotations:
    openshift.io/node-selector: ""
  name: openshift-oauth-apiserver
  labels:
    openshift.io/run-level: "1"
    openshift.io/cluster-monitoring: "true"
`)

func oauthApiserverNsYamlBytes() ([]byte, error) {
	return _oauthApiserverNsYaml, nil
}

func oauthApiserverNsYaml() (*asset, error) {
	bytes, err := oauthApiserverNsYamlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "oauth-apiserver/ns.yaml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _oauthApiserverSaYaml = []byte(`apiVersion: v1
kind: ServiceAccount
metadata:
  namespace: openshift-oauth-apiserver
  name: oauth-apiserver-sa
`)

func oauthApiserverSaYamlBytes() ([]byte, error) {
	return _oauthApiserverSaYaml, nil
}

func oauthApiserverSaYaml() (*asset, error) {
	bytes, err := oauthApiserverSaYamlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "oauth-apiserver/sa.yaml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _oauthApiserverSvcYaml = []byte(`apiVersion: v1
kind: Service
metadata:
  namespace: openshift-oauth-apiserver
  name: api
  annotations:
    service.alpha.openshift.io/serving-cert-secret-name: serving-cert
    prometheus.io/scrape: "true"
    prometheus.io/scheme: https
spec:
  selector:
    apiserver: "true"
  ports:
  - name: https
    port: 443
    targetPort: 8443
`)

func oauthApiserverSvcYamlBytes() ([]byte, error) {
	return _oauthApiserverSvcYaml, nil
}

func oauthApiserverSvcYaml() (*asset, error) {
	bytes, err := oauthApiserverSvcYamlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "oauth-apiserver/svc.yaml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _oauthOpenshiftDeploymentYaml = []byte(`kind: Deployment
apiVersion: apps/v1
metadata:
  namespace: openshift-authentication
  name: oauth-openshift
  labels:
    app: oauth-openshift
spec:
  replicas: 2
  selector:
    matchLabels:
      app: oauth-openshift
  template:
    metadata:
      namespace: openshift-authentication
      name: oauth-openshift
      labels:
        app: oauth-openshift
    spec:
      terminationGracePeriodSeconds: 40
      serviceAccountName: oauth-openshift
      nodeSelector:
        node-role.kubernetes.io/master: ''
      priorityClassName: system-cluster-critical
      affinity:
        podAntiAffinity:
          preferredDuringSchedulingIgnoredDuringExecution:
            - weight: 100
              podAffinityTerm:
                labelSelector:
                  matchLabels:
                    app: oauth-openshift
                topologyKey: kubernetes.io/hostname
      tolerations:
        - key: node-role.kubernetes.io/master
          operator: Exists
          effect: NoSchedule
        - key: node.kubernetes.io/unreachable
          operator: Exists
          effect: NoExecute
          tolerationSeconds: 120
        - key: node.kubernetes.io/not-ready
          operator: Exists
          effect: NoExecute
          tolerationSeconds: 120
      containers:
        - name: oauth-openshift
          image: ${IMAGE}
          command:
            - /bin/bash
            - '-ec'
          args:
            - |
              if [ -s /var/config/system/configmaps/v4-0-config-system-trusted-ca-bundle/ca-bundle.crt ]; then
                  echo "Copying system trust bundle"
                  cp -f /var/config/system/configmaps/v4-0-config-system-trusted-ca-bundle/ca-bundle.crt /etc/pki/ca-trust/extracted/pem/tls-ca-bundle.pem
              fi
              exec oauth-server osinserver \
              --config=/var/config/system/configmaps/v4-0-config-system-cliconfig/v4-0-config-system-cliconfig \
              --v=${LOG_LEVEL}
          ports:
            - name: https
              containerPort: 6443
              protocol: TCP
          volumeMounts:
            - name: v4-0-config-system-session
              readOnly: true
              mountPath: /var/config/system/secrets/v4-0-config-system-session
            - name: v4-0-config-system-cliconfig
              readOnly: true
              mountPath: /var/config/system/configmaps/v4-0-config-system-cliconfig
            - name: v4-0-config-system-serving-cert
              readOnly: true
              mountPath: /var/config/system/secrets/v4-0-config-system-serving-cert
            - name: v4-0-config-system-service-ca
              readOnly: true
              mountPath: /var/config/system/configmaps/v4-0-config-system-service-ca
            - name: v4-0-config-system-router-certs
              readOnly: true
              mountPath: /var/config/system/secrets/v4-0-config-system-router-certs
            - name: v4-0-config-system-ocp-branding-template
              readOnly: true
              mountPath: /var/config/system/secrets/v4-0-config-system-ocp-branding-template
            - name: v4-0-config-system-trusted-ca-bundle
              readOnly: true
              mountPath: /var/config/system/configmaps/v4-0-config-system-trusted-ca-bundle
          readinessProbe:
            httpGet:
              path: /healthz
              port: 6443
              scheme: HTTPS
            timeoutSeconds: 1
            periodSeconds: 10
            successThreshold: 1
            failureThreshold: 3
          livenessProbe:
            httpGet:
              path: /healthz
              port: 6443
              scheme: HTTPS
            initialDelaySeconds: 30
            timeoutSeconds: 1
            periodSeconds: 10
            successThreshold: 1
            failureThreshold: 3
          lifecycle:
            # delay shutdown by 25s to ensure existing connections are drained
            # * 5s for endpoint propagation on delete
            # * 5s for route reload
            # * 15s for the longest running request to finish
            preStop:
              exec:
                command:
                - sleep
                - "25"
          terminationMessagePolicy: FallbackToLogsOnError
          resources:
            requests:
              cpu: 10m
              memory: 50Mi
      volumes:
        - name: v4-0-config-system-session
          secret:
            secretName: v4-0-config-system-session
        - name: v4-0-config-system-cliconfig
          configMap:
            name: v4-0-config-system-cliconfig
        - name: v4-0-config-system-serving-cert
          secret:
            secretName: v4-0-config-system-serving-cert
        - name: v4-0-config-system-service-ca
          configMap:
            name: v4-0-config-system-service-ca
        - name: v4-0-config-system-router-certs
          secret:
            secretName: v4-0-config-system-router-certs
        - name: v4-0-config-system-ocp-branding-template
          secret:
            secretName: v4-0-config-system-ocp-branding-template
        - name: v4-0-config-system-trusted-ca-bundle
          configMap:
            name: v4-0-config-system-trusted-ca-bundle
            optional: true
`)

func oauthOpenshiftDeploymentYamlBytes() ([]byte, error) {
	return _oauthOpenshiftDeploymentYaml, nil
}

func oauthOpenshiftDeploymentYaml() (*asset, error) {
	bytes, err := oauthOpenshiftDeploymentYamlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "oauth-openshift/deployment.yaml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

// Asset loads and returns the asset for the given name.
// It returns an error if the asset could not be found or
// could not be loaded.
func Asset(name string) ([]byte, error) {
	cannonicalName := strings.Replace(name, "\\", "/", -1)
	if f, ok := _bindata[cannonicalName]; ok {
		a, err := f()
		if err != nil {
			return nil, fmt.Errorf("Asset %s can't read by error: %v", name, err)
		}
		return a.bytes, nil
	}
	return nil, fmt.Errorf("Asset %s not found", name)
}

// MustAsset is like Asset but panics when Asset would return an error.
// It simplifies safe initialization of global variables.
func MustAsset(name string) []byte {
	a, err := Asset(name)
	if err != nil {
		panic("asset: Asset(" + name + "): " + err.Error())
	}

	return a
}

// AssetInfo loads and returns the asset info for the given name.
// It returns an error if the asset could not be found or
// could not be loaded.
func AssetInfo(name string) (os.FileInfo, error) {
	cannonicalName := strings.Replace(name, "\\", "/", -1)
	if f, ok := _bindata[cannonicalName]; ok {
		a, err := f()
		if err != nil {
			return nil, fmt.Errorf("AssetInfo %s can't read by error: %v", name, err)
		}
		return a.info, nil
	}
	return nil, fmt.Errorf("AssetInfo %s not found", name)
}

// AssetNames returns the names of the assets.
func AssetNames() []string {
	names := make([]string, 0, len(_bindata))
	for name := range _bindata {
		names = append(names, name)
	}
	return names
}

// _bindata is a table, holding each asset generator, mapped to its name.
var _bindata = map[string]func() (*asset, error){
	"oauth-apiserver/apiserver-clusterrolebinding.yaml": oauthApiserverApiserverClusterrolebindingYaml,
	"oauth-apiserver/cm.yaml":                           oauthApiserverCmYaml,
	"oauth-apiserver/ds.yaml":                           oauthApiserverDsYaml,
	"oauth-apiserver/ns.yaml":                           oauthApiserverNsYaml,
	"oauth-apiserver/sa.yaml":                           oauthApiserverSaYaml,
	"oauth-apiserver/svc.yaml":                          oauthApiserverSvcYaml,
	"oauth-openshift/deployment.yaml":                   oauthOpenshiftDeploymentYaml,
}

// AssetDir returns the file names below a certain
// directory embedded in the file by go-bindata.
// For example if you run go-bindata on data/... and data contains the
// following hierarchy:
//     data/
//       foo.txt
//       img/
//         a.png
//         b.png
// then AssetDir("data") would return []string{"foo.txt", "img"}
// AssetDir("data/img") would return []string{"a.png", "b.png"}
// AssetDir("foo.txt") and AssetDir("notexist") would return an error
// AssetDir("") will return []string{"data"}.
func AssetDir(name string) ([]string, error) {
	node := _bintree
	if len(name) != 0 {
		cannonicalName := strings.Replace(name, "\\", "/", -1)
		pathList := strings.Split(cannonicalName, "/")
		for _, p := range pathList {
			node = node.Children[p]
			if node == nil {
				return nil, fmt.Errorf("Asset %s not found", name)
			}
		}
	}
	if node.Func != nil {
		return nil, fmt.Errorf("Asset %s not found", name)
	}
	rv := make([]string, 0, len(node.Children))
	for childName := range node.Children {
		rv = append(rv, childName)
	}
	return rv, nil
}

type bintree struct {
	Func     func() (*asset, error)
	Children map[string]*bintree
}

var _bintree = &bintree{nil, map[string]*bintree{
	"oauth-apiserver": {nil, map[string]*bintree{
		"apiserver-clusterrolebinding.yaml": {oauthApiserverApiserverClusterrolebindingYaml, map[string]*bintree{}},
		"cm.yaml":                           {oauthApiserverCmYaml, map[string]*bintree{}},
		"ds.yaml":                           {oauthApiserverDsYaml, map[string]*bintree{}},
		"ns.yaml":                           {oauthApiserverNsYaml, map[string]*bintree{}},
		"sa.yaml":                           {oauthApiserverSaYaml, map[string]*bintree{}},
		"svc.yaml":                          {oauthApiserverSvcYaml, map[string]*bintree{}},
	}},
	"oauth-openshift": {nil, map[string]*bintree{
		"deployment.yaml": {oauthOpenshiftDeploymentYaml, map[string]*bintree{}},
	}},
}}

// RestoreAsset restores an asset under the given directory
func RestoreAsset(dir, name string) error {
	data, err := Asset(name)
	if err != nil {
		return err
	}
	info, err := AssetInfo(name)
	if err != nil {
		return err
	}
	err = os.MkdirAll(_filePath(dir, filepath.Dir(name)), os.FileMode(0755))
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(_filePath(dir, name), data, info.Mode())
	if err != nil {
		return err
	}
	err = os.Chtimes(_filePath(dir, name), info.ModTime(), info.ModTime())
	if err != nil {
		return err
	}
	return nil
}

// RestoreAssets restores an asset under the given directory recursively
func RestoreAssets(dir, name string) error {
	children, err := AssetDir(name)
	// File
	if err != nil {
		return RestoreAsset(dir, name)
	}
	// Dir
	for _, child := range children {
		err = RestoreAssets(dir, filepath.Join(name, child))
		if err != nil {
			return err
		}
	}
	return nil
}

func _filePath(dir, name string) string {
	cannonicalName := strings.Replace(name, "\\", "/", -1)
	return filepath.Join(append([]string{dir}, strings.Split(cannonicalName, "/")...)...)
}
