package aibom

import (
	"context"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
)

func newAIBOMObject(namespace, name, modelName string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "aibom.io/v1alpha1",
		"kind":       "AIBOM",
		"metadata": map[string]any{
			"name":      name,
			"namespace": namespace,
		},
		"spec": map[string]any{
			"jobName":          name + "-job",
			"modelName":        modelName,
			"experimentIntent": "training",
			"data": map[string]any{
				"model": map[string]any{
					"name": modelName,
				},
			},
		},
	}}
}

func newTestDynamicClient(objects ...runtime.Object) *dynamicfake.FakeDynamicClient {
	scheme := runtime.NewScheme()
	gvrToListKind := map[schema.GroupVersionResource]string{
		GVR: "AIBOMList",
	}
	return dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, gvrToListKind, objects...)
}

func TestList(t *testing.T) {
	client := newTestDynamicClient(
		newAIBOMObject("ml-a", "run-1", "granite-3.0-8b"),
		newAIBOMObject("ml-b", "run-2", "llama-3-70b"),
	)

	items, err := List(context.Background(), client, "")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d: %+v", len(items), items)
	}
	// Apply sorts by namespace then name.
	if items[0].Namespace != "ml-a" || items[0].Data.Model.Name != "granite-3.0-8b" {
		t.Fatalf("unexpected first item: %+v", items[0])
	}
	if items[0].JobName != "run-1-job" {
		t.Fatalf("expected jobName to round-trip, got %+v", items[0])
	}
}

func TestListNamespaced(t *testing.T) {
	client := newTestDynamicClient(
		newAIBOMObject("ml-a", "run-1", "granite-3.0-8b"),
		newAIBOMObject("ml-b", "run-2", "llama-3-70b"),
	)

	items, err := List(context.Background(), client, "ml-b")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(items) != 1 || items[0].Name != "run-2" {
		t.Fatalf("expected only run-2 in ml-b, got %+v", items)
	}
}

func TestGet(t *testing.T) {
	client := newTestDynamicClient(newAIBOMObject("ml-a", "run-1", "granite-3.0-8b"))

	a, err := Get(context.Background(), client, "ml-a", "run-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if a.Name != "run-1" || a.Data.Model.Name != "granite-3.0-8b" {
		t.Fatalf("unexpected result: %+v", a)
	}
}

func TestGetNotFound(t *testing.T) {
	client := newTestDynamicClient()
	if _, err := Get(context.Background(), client, "ml-a", "missing"); err == nil {
		t.Fatal("expected error for missing AIBOM, got nil")
	}
}
