package v1alpha1_test

import (
	"os"
	"path/filepath"
	"testing"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	sigsyaml "sigs.k8s.io/yaml"
)

// A ServiceAccount applied with no spec key must still default spec.state to
// Active. CRD defaulting fills a nested default only when the parent object is
// present, so the spec property itself needs a default of {}.
func TestServiceAccountSpecDefaultsToEmptyObject(t *testing.T) {
	crdPath := filepath.Join(repoRoot(t), "config", "crd", "bases", "iam", "iam.miloapis.com_serviceaccounts.yaml")
	raw, err := os.ReadFile(crdPath)
	if err != nil {
		t.Fatalf("reading CRD: %v", err)
	}

	var crd apiextensionsv1.CustomResourceDefinition
	if err := sigsyaml.Unmarshal(raw, &crd); err != nil {
		t.Fatalf("unmarshaling CRD: %v", err)
	}
	if len(crd.Spec.Versions) == 0 {
		t.Fatal("CRD has no versions")
	}

	schema := crd.Spec.Versions[0].Schema.OpenAPIV3Schema
	specProp, ok := schema.Properties["spec"]
	if !ok {
		t.Fatal("schema missing spec property")
	}
	if specProp.Default == nil {
		t.Fatal("spec property has no default; an absent spec would never default spec.state")
	}
	if got := string(specProp.Default.Raw); got != "{}" {
		t.Errorf("spec default = %s, want {}", got)
	}

	stateProp, ok := specProp.Properties["state"]
	if !ok {
		t.Fatal("spec schema missing state property")
	}
	if stateProp.Default == nil {
		t.Fatal("spec.state has no default")
	}
	if got := string(stateProp.Default.Raw); got != `"Active"` {
		t.Errorf("spec.state default = %s, want \"Active\"", got)
	}
}
