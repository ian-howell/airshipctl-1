// Copyright 2019 The Kubernetes Authors.
// SPDX-License-Identifier: Apache-2.0

package replacement_test

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"sigs.k8s.io/kustomize/kyaml/kio"
	"sigs.k8s.io/yaml"

	"opendev.org/airship/airshipctl/pkg/document/plugin/replacement"
	plugtypes "opendev.org/airship/airshipctl/pkg/document/plugin/types"
)

func samplePlugin(t *testing.T) plugtypes.Plugin {
	cfg := make(map[string]interface{})
	conf := `
apiVersion: airshipit.org/v1alpha1
kind: ReplacementTransformer
metadata:
  name: notImportantHere
replacements:
- source:
    value: nginx:newtag
  target:
    objref:
      kind: Deployment
    fieldrefs:
    - spec.template.spec.containers[name=nginx-latest].image`

	err := yaml.Unmarshal([]byte(conf), &cfg)
	require.NoError(t, err)
	plugin, err := replacement.New(cfg)
	require.NoError(t, err)
	return plugin
}

func TestMalformedInput(t *testing.T) {
	plugin := samplePlugin(t)
	err := plugin.Run(strings.NewReader("--"), &bytes.Buffer{})
	assert.Error(t, err)
}

func TestDuplicatedResources(t *testing.T) {
	plugin := samplePlugin(t)
	err := plugin.Run(strings.NewReader(`
apiVersion: v1
kind: Pod
metadata:
  name: pod
spec:
  containers:
  - name: myapp-container
    image: busybox
---
apiVersion: v1
kind: Pod
metadata:
  name: pod
spec:
  containers:
  - name: myapp-container
    image: busybox
 `), &bytes.Buffer{})
	assert.Error(t, err)
}

var testCases = []struct {
	cfg         string
	in          string
	expectedOut string
	expectedErr string
}{
	{
		cfg: `
apiVersion: airshipit.org/v1alpha1
kind: ReplacementTransformer
metadata:
  name: notImportantHere
replacements:
- source:
    value: nginx:newtag
  target:
    objref:
      kind: Deployment
    fieldrefs:
    - spec.template.spec.containers[name=nginx.latest].image
- source:
    value: postgres:latest
  target:
    objref:
      kind: Deployment
    fieldrefs:
    - spec.template.spec.containers.3.image
`,

		in: `
apiVersion: v1
group: apps
kind: Deployment
metadata:
  name: deploy1
spec:
  template:
    spec:
      containers:
      - image: nginx:1.7.9
        name: nginx-tagged
      - image: nginx:latest
        name: nginx.latest
      - image: foobar:1
        name: replaced-with-digest
      - image: postgres:1.8.0
        name: postgresdb
      initContainers:
      - image: nginx
        name: nginx-notag
      - image: nginx@sha256:111111111111111111
        name: nginx-sha256
      - image: alpine:1.8.0
        name: init-alpine
`,
		expectedOut: `apiVersion: v1
group: apps
kind: Deployment
metadata:
  name: deploy1
spec:
  template:
    spec:
      containers:
      - image: nginx:1.7.9
        name: nginx-tagged
      - image: nginx:newtag
        name: nginx.latest
      - image: foobar:1
        name: replaced-with-digest
      - image: postgres:latest
        name: postgresdb
      initContainers:
      - image: nginx
        name: nginx-notag
      - image: nginx@sha256:111111111111111111
        name: nginx-sha256
      - image: alpine:1.8.0
        name: init-alpine
`,
	},
	{
		cfg: `
apiVersion: airshipit.org/v1alpha1
kind: ReplacementTransformer
metadata:
  name: notImportantHere
replacements:
- source:
    value: 1.17.0
  target:
    objref:
      kind: Deployment
    fieldrefs:
    - spec.template.spec.containers[name=nginx-tagged].image%1.7.9%
`,

		in: `
apiVersion: v1
group: apps
kind: Deployment
metadata:
  name: deploy1
spec:
  template:
    spec:
      containers:
      - image: nginx:1.7.9
        name: nginx-tagged
`,
		expectedOut: `apiVersion: v1
group: apps
kind: Deployment
metadata:
  name: deploy1
spec:
  template:
    spec:
      containers:
      - image: nginx:1.17.0
        name: nginx-tagged
`,
	},

	{
		cfg: `
apiVersion: airshipit.org/v1alpha1
kind: ReplacementTransformer
metadata:
  name: notImportantHere
replacements:
- source:
    objref:
      kind: Pod
      name: pod
    fieldref: spec.containers
  target:
    objref:
      kind: Deployment
    fieldrefs:
    - spec.template.spec.containers`,
		in: `
apiVersion: v1
kind: Pod
metadata:
  name: pod
spec:
  containers:
  - image: busybox
    name: myapp-container
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: deploy2
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: deploy3
`,
		expectedOut: `apiVersion: v1
kind: Pod
metadata:
  name: pod
spec:
  containers:
  - image: busybox
    name: myapp-container
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: deploy2
spec:
  template:
    spec:
      containers:
      - image: busybox
        name: myapp-container
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: deploy3
spec:
  template:
    spec:
      containers:
      - image: busybox
        name: myapp-container
`,
	},
	{
		cfg: `
apiVersion: airshipit.org/v1alpha1
kind: ReplacementTransformer
metadata:
  name: notImportantHere
replacements:
- source:
    objref:
      kind: ConfigMap
      name: cm
    fieldref: data.HOSTNAME
  target:
    objref:
      kind: Deployment
    fieldrefs:
    - spec.template.spec.containers[image=debian].args.0
    - spec.template.spec.containers[name=busybox].args.1
- source:
    objref:
      kind: ConfigMap
      name: cm
    fieldref: data.PORT
  target:
    objref:
      kind: Deployment
    fieldrefs:
    - spec.template.spec.containers[image=debian].args.1
    - spec.template.spec.containers[name=busybox].args.2`,
		in: `
apiVersion: apps/v1
kind: Deployment
metadata:
  labels:
    foo: bar
  name: deploy
spec:
  template:
    metadata:
      labels:
        foo: bar
    spec:
      containers:
      - args:
        - HOSTNAME
        - PORT
        command:
        - printenv
        image: debian
        name: command-demo-container
      - args:
        - echo
        - HOSTNAME
        - PORT
        image: busybox:latest
        name: busybox
---
apiVersion: v1
data:
  HOSTNAME: example.com
  PORT: 8080
kind: ConfigMap
metadata:
  name: cm`,
		expectedOut: `apiVersion: apps/v1
kind: Deployment
metadata:
  labels:
    foo: bar
  name: deploy
spec:
  template:
    metadata:
      labels:
        foo: bar
    spec:
      containers:
      - args:
        - example.com
        - 8080
        command:
        - printenv
        image: debian
        name: command-demo-container
      - args:
        - echo
        - example.com
        - 8080
        image: busybox:latest
        name: busybox
---
apiVersion: v1
data:
  HOSTNAME: example.com
  PORT: 8080
kind: ConfigMap
metadata:
  name: cm
`,
	},
	{
		cfg: `
apiVersion: airshipit.org/v1alpha1
kind: ReplacementTransformer
metadata:
  name: notImportantHere
replacements:
- source:
    value: regexedtag
  target:
    objref:
      kind: Deployment
    fieldrefs:
    - spec.template.spec.containers[name=nginx-latest].image%TAG%
- source:
    value: postgres:latest
  target:
    objref:
      kind: Deployment
    fieldrefs:
    - spec.template.spec.containers.3.image`,
		in: `
apiVersion: v1
group: apps
kind: Deployment
metadata:
  name: deploy1
spec:
  template:
    spec:
      containers:
      - image: nginx:1.7.9
        name: nginx-tagged
      - image: nginx:TAG
        name: nginx-latest
      - image: foobar:1
        name: replaced-with-digest
      - image: postgres:1.8.0
        name: postgresdb
      initContainers:
      - image: nginx
        name: nginx-notag
      - image: nginx@sha256:111111111111111111
        name: nginx-sha256
      - image: alpine:1.8.0
        name: init-alpine`,
		expectedOut: `apiVersion: v1
group: apps
kind: Deployment
metadata:
  name: deploy1
spec:
  template:
    spec:
      containers:
      - image: nginx:1.7.9
        name: nginx-tagged
      - image: nginx:regexedtag
        name: nginx-latest
      - image: foobar:1
        name: replaced-with-digest
      - image: postgres:latest
        name: postgresdb
      initContainers:
      - image: nginx
        name: nginx-notag
      - image: nginx@sha256:111111111111111111
        name: nginx-sha256
      - image: alpine:1.8.0
        name: init-alpine
`,
	},
	{
		cfg: `
apiVersion: airshipit.org/v1alpha1
kind: ReplacementTransformer
metadata:
  name: notImportantHere
replacements:
- source:
    objref:
      kind: Pod
      name: pod1
  target:
    objref:
      kind: Pod
      name: pod2
    fieldrefs:
    - spec.non.existent.field`,
		in: `
apiVersion: v1
kind: Pod
metadata:
  name: pod1
spec:
  containers:
  - image: busybox
    name: myapp-container
---
apiVersion: v1
kind: Pod
metadata:
  name: pod2
spec:
  containers:
  - image: busybox
    name: myapp-container`,
		expectedOut: `apiVersion: v1
kind: Pod
metadata:
  name: pod1
spec:
  containers:
  - image: busybox
    name: myapp-container
---
apiVersion: v1
kind: Pod
metadata:
  name: pod2
spec:
  containers:
  - image: busybox
    name: myapp-container
  non:
    existent:
      field: pod1
`,
	},
	{
		cfg: `
apiVersion: airshipit.org/v1alpha1
kind: ReplacementTransformer
metadata:
  name: notImportantHere
replacements:
- source:
    objref:
      kind: Pod
      name: pod
    fieldref: spec.containers[0]
  target:
    objref:
      kind: Deployment
    fieldrefs:
    - spec.template.spec.containers[name=myapp-container]`,
		in: `
apiVersion: v1
kind: Pod
metadata:
  name: pod
spec:
  containers:
  - image: repl
    name: repl
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: deploy2
spec:
  template:
    spec:
      containers:
      - image: busybox
        name: myapp-container
`,
		expectedOut: `apiVersion: v1
kind: Pod
metadata:
  name: pod
spec:
  containers:
  - image: repl
    name: repl
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: deploy2
spec:
  template:
    spec:
      containers:
      - image: repl
        name: repl
`,
	},
	{
		cfg: `
apiVersion: airshipit.org/v1alpha1
kind: ReplacementTransformer
metadata:
  name: test-for-numeric-conversion
replacements:
- source:
    objref:
      kind: Pod
      name: pod
    fieldref: spec.containers[0].image
  target:
    objref:
      kind: Deployment
    fieldrefs:
    - spec.template.spec.containers[name=myapp-container].image%TAG%`,
		in: `
apiVersion: v1
kind: Pod
metadata:
  name: pod
spec:
  containers:
  - image: 12345
    name: repl
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: deploy2
spec:
  template:
    spec:
      containers:
      - image: busybox:TAG
        name: myapp-container
`,
		expectedOut: `apiVersion: v1
kind: Pod
metadata:
  name: pod
spec:
  containers:
  - image: 12345
    name: repl
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: deploy2
spec:
  template:
    spec:
      containers:
      - image: busybox:12345
        name: myapp-container
`,
	},
	{
		cfg: `
apiVersion: airshipit.org/v1alpha1
kind: ReplacementTransformer
metadata:
  name: notImportantHere
replacements:
- source:
    objref:
      kind: Pod
  target:
    objref:
      kind: Deployment
    fieldrefs:
    - spec.template.spec.containers[name=nginx-latest].image`,
		in: `
apiVersion: v1
kind: Pod
metadata:
  name: pod1
spec:
  containers:
  - name: myapp-container
    image: busybox
---
apiVersion: v1
kind: Pod
metadata:
  name: pod2
spec:
  containers:
  - name: myapp-container
    image: busybox`,
		expectedErr: "found more than one resources matching identified by Gvk: ~G_~V_Pod",
	},
	{
		cfg: `
apiVersion: airshipit.org/v1alpha1
kind: ReplacementTransformer
metadata:
  name: notImportantHere
replacements:
- source:
    objref:
      kind: Pod
      name: doesNotExists
      namespace: default
  target:
    objref:
      kind: Deployment
    fieldrefs:
    - spec.template.spec.containers[name=nginx-latest].image`,
		in: `apiVersion: v1
kind: Pod
metadata:
  name: pod1`,
		expectedErr: "failed to find any source resources identified by " +
			"Gvk: ~G_~V_Pod Name: doesNotExists Namespace: default",
	},
	{
		cfg: `
apiVersion: airshipit.org/v1alpha1
kind: ReplacementTransformer
metadata:
  name: notImportantHere
replacements:
- source:
    objref:
      kind: Pod
      name: pod1
  target:
    objref:
      kind: Deployment
    fieldrefs:
    - spec.template.spec.containers[name=nginx-latest].image`,
		in: `
apiVersion: v1
kind: Pod
metadata:
  name: pod1
spec:
  containers:
  - name: myapp-container
    image: busybox`,
		expectedErr: "failed to find any target resources identified by Gvk: ~G_~V_Deployment",
	},
	{
		cfg: `
apiVersion: airshipit.org/v1alpha1
kind: ReplacementTransformer
metadata:
  name: notImportantHere
replacements:
- source:
    objref:
      kind: Pod
      name: pod1
  target:
    objref:
      kind: Pod
      name: pod2
    fieldrefs:
    - labels.somelabel.key1.subkey1`,
		in: `
apiVersion: v1
kind: Pod
metadata:
  name: pod1
spec:
  containers:
  - name: myapp-container
    image: busybox
---
apiVersion: v1
kind: Pod
labels:
  somelabel: 'some string value'
metadata:
  name: pod2
spec:
  containers:
  - name: myapp-container
    image: busybox`,
		expectedErr: `"some string value" is not expected be a primitive type`,
	},
	{
		cfg: `
apiVersion: airshipit.org/v1alpha1
kind: ReplacementTransformer
metadata:
  name: notImportantHere
replacements:
- source:
    objref:
      kind: Pod
      name: pod1
  target:
    objref:
      kind: Pod
      name: pod2
    fieldrefs:
    - labels.somelabel[subkey1=val1]`,
		in: `
apiVersion: v1
kind: Pod
metadata:
  name: pod1
spec:
  containers:
  - name: myapp-container
    image: busybox
---
apiVersion: v1
kind: Pod
labels:
  somelabel: 'some string value'
metadata:
  name: pod2
spec:
  containers:
  - name: myapp-container
    image: busybox`,
		expectedErr: `"some string value" is not expected be a primitive type`,
	},
	{
		cfg: `
apiVersion: airshipit.org/v1alpha1
kind: ReplacementTransformer
metadata:
  name: notImportantHere
replacements:
- source:
    objref:
      kind: Pod
      name: pod1
  target:
    objref:
      kind: Pod
      name: pod2
    fieldrefs:
    - spec[subkey1=val1]`,
		in: `
apiVersion: v1
kind: Pod
metadata:
  name: pod1
spec:
  containers:
  - name: myapp-container
    image: busybox
---
apiVersion: v1
kind: Pod
labels:
  somelabel: 'some string value'
metadata:
  name: pod2
spec:
  containers:
  - name: myapp-container
    image: busybox`,
		expectedErr: "map[string]interface {}{\"containers\":[]interface " +
			"{}{map[string]interface {}{\"image\":\"busybox\", \"name\":\"myapp-container\"}}} " +
			"is not expected be a primitive type",
	},
	{
		cfg: `
apiVersion: airshipit.org/v1alpha1
kind: ReplacementTransformer
metadata:
  name: notImportantHere
replacements:
- source:
    objref:
      kind: Pod
      name: pod1
  target:
    objref:
      kind: Pod
      name: pod2
    fieldrefs:
    - spec.containers.10`,
		in: `
apiVersion: v1
kind: Pod
metadata:
  name: pod1
spec:
  containers:
  - name: myapp-container
    image: busybox
---
apiVersion: v1
kind: Pod
labels:
  somelabel: 'some string value'
metadata:
  name: pod2
spec:
  containers:
  - name: myapp-container
    image: busybox`,
		expectedErr: "array index out of bounds: index 10, length 1",
	},
	{
		cfg: `
apiVersion: airshipit.org/v1alpha1
kind: ReplacementTransformer
metadata:
  name: notImportantHere
replacements:
- source:
    objref:
      kind: Pod
      name: pod1
  target:
    objref:
      kind: Pod
      name: pod2
    fieldrefs:
    - spec.containers.notInteger.name`,
		in: `
apiVersion: v1
kind: Pod
metadata:
  name: pod1
spec:
  containers:
  - name: myapp-container
    image: busybox
---
apiVersion: v1
kind: Pod
labels:
  somelabel: 'some string value'
metadata:
  name: pod2
spec:
  containers:
  - name: myapp-container
    image: busybox`,
		expectedErr: `strconv.Atoi: parsing "notInteger": invalid syntax`,
	},
	{
		cfg: `
apiVersion: airshipit.org/v1alpha1
kind: ReplacementTransformer
metadata:
  name: notImportantHere
replacements:
- source:
    objref:
      kind: Pod
      name: pod1
  target:
    objref:
      kind: Deployment
    fieldrefs:
    - spec.template.spec.containers%TAG%`,
		in: `
apiVersion: v1
kind: Pod
metadata:
  name: pod1
spec:
  containers:
  - name: myapp-container
    image: busybox
---
group: apps
apiVersion: v1
kind: Deployment
metadata:
  name: deploy1
spec:
  template:
    spec:
      containers:
      - image: nginx:TAG
        name: nginx-latest`,
		expectedErr: "pattern-based substitution can only be applied to string target fields",
	},
	{
		cfg: `
apiVersion: airshipit.org/v1alpha1
kind: ReplacementTransformer
metadata:
  name: notImportantHere
replacements:
- source:
    objref:
      kind: Pod
      name: pod1
  target:
    objref:
      kind: Deployment
    fieldrefs:
    - spec.template.spec.containers[name=nginx-latest].image%TAG%`,
		in: `
apiVersion: v1
kind: Pod
metadata:
  name: pod1
spec:
  containers:
  - name: myapp-container
    image: busybox
---
group: apps
apiVersion: v1
kind: Deployment
metadata:
  name: deploy1
spec:
  template:
    spec:
      containers:
      - image: nginx:latest
        name: nginx-latest`,
		expectedErr: "pattern 'TAG' is defined in configuration but was not found in target value nginx:latest",
	},
	{
		cfg: `
apiVersion: airshipit.org/v1alpha1
kind: ReplacementTransformer
metadata:
  name: notImportantHere
replacements:
- source:
    value: "12345678"
  target:
    objref:
      kind: KubeadmControlPlane
    fieldrefs:
    - spec.kubeadmConfigSpec.files[path=konfigadm].content%{k8s-version}%`,
		in: `
kind: KubeadmControlPlane
metadata:
  name: cluster-controlplane
spec:
  infrastructureTemplate:
    apiVersion: infrastructure.cluster.x-k8s.io/v1alpha3
    kind: Metal3MachineTemplate
    name: $(cluster-name)
  kubeadmConfigSpec:
    files:
    - content: |
        kubernetes:
          version: {k8s-version}
        container_runtime:
          type: docker
      owner: root:root
      path: konfigadm_bug_
      permissions: "0640"`,
		expectedOut: `kind: KubeadmControlPlane
metadata:
  name: cluster-controlplane
spec:
  infrastructureTemplate:
    apiVersion: infrastructure.cluster.x-k8s.io/v1alpha3
    kind: Metal3MachineTemplate
    name: $(cluster-name)
  kubeadmConfigSpec:
    files:
    - content: |
        kubernetes:
          version: {k8s-version}
        container_runtime:
          type: docker
      owner: root:root
      path: konfigadm_bug_
      permissions: "0640"
`,
	},
}

func TestReplacementTransformer(t *testing.T) {
	for _, tc := range testCases {
		cfg := make(map[string]interface{})
		err := yaml.Unmarshal([]byte(tc.cfg), &cfg)
		require.NoError(t, err)
		plugin, err := replacement.New(cfg)
		require.NoError(t, err)

		buf := &bytes.Buffer{}
		err = plugin.Run(strings.NewReader(tc.in), buf)
		errString := ""
		if err != nil {
			errString = err.Error()
		}
		assert.Equal(t, tc.expectedErr, errString)
		assert.Equal(t, tc.expectedOut, buf.String())
	}
}

func TestExec(t *testing.T) {
	// TODO (dukov) Remove this once we migrate to new kustomize plugin approach
	// NOTE (dukov) we need this since error format is different for new kustomize plugins
	testCases[11].expectedErr = "wrong Node Kind for labels.somelabel expected: " +
		"MappingNode was ScalarNode: value: {'some string value'}"
	testCases[12].expectedErr = "wrong Node Kind for labels.somelabel expected: " +
		"SequenceNode was ScalarNode: value: {'some string value'}"
	testCases[13].expectedErr = "wrong Node Kind for spec expected: " +
		"SequenceNode was MappingNode: value: {containers:\n- name: myapp-container\n  image: busybox}"
	testCases[15].expectedErr = "wrong Node Kind for spec.containers expected: " +
		"MappingNode was SequenceNode: value: {- name: myapp-container\n  image: busybox}"
	testCases[16].expectedErr = "wrong Node Kind for  expected: " +
		"ScalarNode was MappingNode: value: {image: nginx:TAG\nname: nginx-latest}"

	for i, tc := range testCases {
		tc := tc
		t.Run(fmt.Sprintf("Test Case %d", i), func(t *testing.T) {
			cfg := make(map[string]interface{})
			err := yaml.Unmarshal([]byte(tc.cfg), &cfg)
			require.NoError(t, err)
			plugin, err := replacement.New(cfg)
			require.NoError(t, err)

			buf := &bytes.Buffer{}

			p := kio.Pipeline{
				Inputs:  []kio.Reader{&kio.ByteReader{Reader: bytes.NewBufferString(tc.in)}},
				Filters: []kio.Filter{plugin},
				Outputs: []kio.Writer{kio.ByteWriter{Writer: buf}},
			}
			err = p.Execute()

			errString := ""
			if err != nil {
				errString = err.Error()
			}

			assert.Equal(t, tc.expectedErr, errString)
			assert.Equal(t, tc.expectedOut, buf.String())
		})
	}
}
