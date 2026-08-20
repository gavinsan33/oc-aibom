package aibom

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

// GVR identifies the AIBOM custom resource. There is no generated typed
// clientset for it, so this package talks to the API via the dynamic client
// against this GroupVersionResource.
var GVR = schema.GroupVersionResource{
	Group:    "aibom.io",
	Version:  "v1alpha1",
	Resource: "aiboms",
}

// fromUnstructured decodes a raw *unstructured.Unstructured AIBOM object
// into the typed AIBOM struct. spec.data is preserved-unknown-fields on the
// CRD, so this best-effort JSON round-trip tolerates fields the schema
// doesn't know about yet.
func fromUnstructured(obj *unstructured.Unstructured) (AIBOM, error) {
	spec, _, err := unstructured.NestedMap(obj.Object, "spec")
	if err != nil {
		return AIBOM{}, fmt.Errorf("reading spec: %w", err)
	}

	raw, err := json.Marshal(spec)
	if err != nil {
		return AIBOM{}, fmt.Errorf("marshaling spec: %w", err)
	}

	var a AIBOM
	if err := json.Unmarshal(raw, &a); err != nil {
		return AIBOM{}, fmt.Errorf("unmarshaling spec: %w", err)
	}
	a.Name = obj.GetName()
	a.Namespace = obj.GetNamespace()
	return a, nil
}

// List returns AIBOM objects in namespace, or across all namespaces if
// namespace is empty.
func List(ctx context.Context, client dynamic.Interface, namespace string) ([]AIBOM, error) {
	var ri dynamic.ResourceInterface
	if namespace == "" {
		ri = client.Resource(GVR)
	} else {
		ri = client.Resource(GVR).Namespace(namespace)
	}

	list, err := ri.List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing aiboms: %w", err)
	}

	results := make([]AIBOM, 0, len(list.Items))
	for i := range list.Items {
		a, err := fromUnstructured(&list.Items[i])
		if err != nil {
			return nil, fmt.Errorf("decoding aibom %s/%s: %w", list.Items[i].GetNamespace(), list.Items[i].GetName(), err)
		}
		results = append(results, a)
	}

	sort.Slice(results, func(i, j int) bool {
		if results[i].Namespace != results[j].Namespace {
			return results[i].Namespace < results[j].Namespace
		}
		return results[i].Name < results[j].Name
	})

	return results, nil
}

// Get fetches a single named AIBOM in namespace.
func Get(ctx context.Context, client dynamic.Interface, namespace, name string) (AIBOM, error) {
	obj, err := client.Resource(GVR).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return AIBOM{}, fmt.Errorf("getting aibom %s/%s: %w", namespace, name, err)
	}
	return fromUnstructured(obj)
}
