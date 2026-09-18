package aibom

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func signedAIBOM(t *testing.T, namespace, name string, pub ed25519.PublicKey, sig []byte, data map[string]any) AIBOM {
	t.Helper()
	return AIBOM{
		Name:               name,
		Namespace:          namespace,
		Signature:          base64.StdEncoding.EncodeToString(sig),
		SignaturePublicKey: base64.StdEncoding.EncodeToString(pub),
		RawData:            data,
	}
}

func newConfigMapObject(namespace, name string, data map[string]string) *unstructured.Unstructured {
	untypedData := map[string]any{}
	for k, v := range data {
		untypedData[k] = v
	}
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1",
		"kind":       "ConfigMap",
		"metadata": map[string]any{
			"name":      name,
			"namespace": namespace,
		},
		"data": untypedData,
	}}
}

func TestVerify_Unsigned(t *testing.T) {
	a := AIBOM{Name: "run-1", Namespace: "ml-a", RawData: map[string]any{"model": map[string]any{"name": "x"}}}
	result := Verify(context.Background(), nil, a)
	if result.Status != VerifyUnsigned {
		t.Fatalf("expected VerifyUnsigned, got %v (%s)", result.Status, result.Detail)
	}
}

func TestVerify_ValidWithClusterKeyMatch(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	data := map[string]any{"model": map[string]any{"name": "granite-3.0-8b"}, "training": map[string]any{"learning_rate": 2e-5}}
	canonical, err := canonicalizeData(data)
	if err != nil {
		t.Fatal(err)
	}
	sig := ed25519.Sign(priv, canonical)
	a := signedAIBOM(t, "ml-a", "run-1", pub, sig, data)

	client := newTestDynamicClient(newConfigMapObject("ml-a", signingPublicKeyConfigMapName, map[string]string{
		signingPublicKeyDataKey: a.SignaturePublicKey,
	}))

	result := Verify(context.Background(), client, a)
	if result.Status != VerifyValid {
		t.Fatalf("expected VerifyValid, got %v (%s)", result.Status, result.Detail)
	}
}

func TestVerify_UnconfirmedWhenConfigMapMissing(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	data := map[string]any{"model": map[string]any{"name": "granite-3.0-8b"}}
	canonical, err := canonicalizeData(data)
	if err != nil {
		t.Fatal(err)
	}
	sig := ed25519.Sign(priv, canonical)
	a := signedAIBOM(t, "ml-a", "run-1", pub, sig, data)

	client := newTestDynamicClient() // no ConfigMap present

	result := Verify(context.Background(), client, a)
	if result.Status != VerifyUnconfirmed {
		t.Fatalf("expected VerifyUnconfirmed, got %v (%s)", result.Status, result.Detail)
	}
}

func TestVerify_UnconfirmedWithNoClusterAtAll(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	data := map[string]any{"model": map[string]any{"name": "granite-3.0-8b"}}
	canonical, err := canonicalizeData(data)
	if err != nil {
		t.Fatal(err)
	}
	sig := ed25519.Sign(priv, canonical)
	a := signedAIBOM(t, "ml-a", "run-1", pub, sig, data)

	result := Verify(context.Background(), nil, a)
	if result.Status != VerifyUnconfirmed {
		t.Fatalf("expected VerifyUnconfirmed, got %v (%s)", result.Status, result.Detail)
	}
}

func TestVerify_KeyMismatch(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	otherPub, _, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	data := map[string]any{"model": map[string]any{"name": "granite-3.0-8b"}}
	canonical, err := canonicalizeData(data)
	if err != nil {
		t.Fatal(err)
	}
	sig := ed25519.Sign(priv, canonical)
	a := signedAIBOM(t, "ml-a", "run-1", pub, sig, data)

	// Cluster's published key doesn't match the one embedded in the AIBOM --
	// e.g. the key was rotated after this AIBOM was signed.
	client := newTestDynamicClient(newConfigMapObject("ml-a", signingPublicKeyConfigMapName, map[string]string{
		signingPublicKeyDataKey: base64.StdEncoding.EncodeToString(otherPub),
	}))

	result := Verify(context.Background(), client, a)
	if result.Status != VerifyKeyMismatch {
		t.Fatalf("expected VerifyKeyMismatch, got %v (%s)", result.Status, result.Detail)
	}
}

func TestVerify_InvalidWhenDataTampered(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	original := map[string]any{"model": map[string]any{"name": "granite-3.0-8b"}}
	canonical, err := canonicalizeData(original)
	if err != nil {
		t.Fatal(err)
	}
	sig := ed25519.Sign(priv, canonical)

	tampered := map[string]any{"model": map[string]any{"name": "a-different-model"}}
	a := signedAIBOM(t, "ml-a", "run-1", pub, sig, tampered)

	result := Verify(context.Background(), nil, a)
	if result.Status != VerifyInvalid {
		t.Fatalf("expected VerifyInvalid, got %v (%s)", result.Status, result.Detail)
	}
}

func TestVerify_InvalidWhenSignatureNotBase64(t *testing.T) {
	pub, _, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	a := AIBOM{
		Name:               "run-1",
		Namespace:          "ml-a",
		Signature:          "not-valid-base64!!!",
		SignaturePublicKey: base64.StdEncoding.EncodeToString(pub),
		RawData:            map[string]any{"model": map[string]any{"name": "x"}},
	}
	result := Verify(context.Background(), nil, a)
	if result.Status != VerifyInvalid {
		t.Fatalf("expected VerifyInvalid, got %v (%s)", result.Status, result.Detail)
	}
}

func TestVerify_InvalidWhenPublicKeyWrongSize(t *testing.T) {
	a := AIBOM{
		Name:               "run-1",
		Namespace:          "ml-a",
		Signature:          base64.StdEncoding.EncodeToString([]byte("not-a-real-signature-but-64-bytes-long-000000000000000000000000")),
		SignaturePublicKey: base64.StdEncoding.EncodeToString([]byte("too-short")),
		RawData:            map[string]any{"model": map[string]any{"name": "x"}},
	}
	result := Verify(context.Background(), nil, a)
	if result.Status != VerifyInvalid {
		t.Fatalf("expected VerifyInvalid, got %v (%s)", result.Status, result.Detail)
	}
}

func TestCanonicalizeData_MatchesGoJCSReferenceOutput(t *testing.T) {
	// Same fixture as aibom-webhook-service's
	// test_sign_aibom_matches_go_jcs_reference_output, run through the other
	// side of the same cross-language check: this repo's Go jcs output
	// should match that repo's Python rfc8785 output byte-for-byte, since
	// both sign/verify against the same RFC 8785 canonical form.
	data := map[string]any{
		"model":    map[string]any{"name": "tinyllama-1.1b-chat", "quantization": nil},
		"training": map[string]any{"learning_rate": 2e-5, "epochs": 3, "random_seed": 42},
		"tags":     []any{"sft", "lora"},
		"dirty":    false,
	}
	got, err := canonicalizeData(data)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"dirty":false,"model":{"name":"tinyllama-1.1b-chat","quantization":null},` +
		`"tags":["sft","lora"],"training":{"epochs":3,"learning_rate":0.00002,"random_seed":42}}`
	if string(got) != want {
		t.Fatalf("canonicalizeData mismatch:\n got:  %s\n want: %s", got, want)
	}
}
