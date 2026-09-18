package aibom

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/gowebpki/jcs"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

// VerifyStatus is the outcome of checking an AIBOM's signature. See
// aibom-webhook-service's CLAUDE.md, "Compiled AIBOM Signing", for the
// full design this verifies against.
type VerifyStatus int

const (
	// VerifyUnsigned means the AIBOM has no signature at all -- e.g. it was
	// created before the signing feature shipped, or in a namespace whose
	// aibom-workload-namespace chart install predates the
	// aibom-compiled-signing-key Secret. Not itself evidence of tampering.
	VerifyUnsigned VerifyStatus = iota

	// VerifyValid means the signature matches spec.data, and the embedded
	// public key matches the one published cluster-side in the
	// aibom-compiled-signing-public-key ConfigMap. This is the only status
	// that should be presented to a user as "verified."
	VerifyValid

	// VerifyInvalid means the signature does not match spec.data under the
	// embedded public key -- spec.data was altered after signing, or the
	// signature/key was corrupted or fabricated.
	VerifyInvalid

	// VerifyKeyMismatch means the signature is internally consistent (it
	// verifies under the embedded SignaturePublicKey), but that key does not
	// match the one currently published in the cluster's
	// aibom-compiled-signing-public-key ConfigMap. On its own this proves
	// nothing was tampered with *after* signing, but it does mean the
	// embedded key alone isn't trustworthy -- an attacker who could forge
	// spec.data could just as easily sign it with their own keypair and
	// embed their own public key alongside it. It can also happen benignly
	// after a legitimate key rotation (see CLAUDE.md's Compiled AIBOM
	// Signing "Scope and known limitations").
	VerifyKeyMismatch

	// VerifyUnconfirmed means the signature verifies under the embedded
	// public key, but the cluster-side anchor couldn't be checked (no RBAC,
	// no network, the ConfigMap doesn't exist yet, or this AIBOM is being
	// checked from an archived export with no cluster reachable at all).
	// This is weaker than VerifyValid: it only proves internal consistency,
	// not that the embedded key is the one the cluster actually published.
	VerifyUnconfirmed
)

// VerifyResult is the outcome of Verify, along with a human-readable Detail
// explaining why.
type VerifyResult struct {
	Status VerifyStatus
	Detail string
}

var configMapGVR = schema.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"}

const (
	signingPublicKeyConfigMapName = "aibom-compiled-signing-public-key"
	signingPublicKeyDataKey       = "ed25519-public-key"
)

// Verify checks a's signature against its own data, and -- when client and
// namespace are non-empty -- cross-checks the embedded public key against
// the cluster's published anchor. Pass an empty namespace (or a nil client)
// to skip the cluster cross-check entirely, e.g. when checking an AIBOM read
// from an archived export with no cluster available; the result is capped
// at VerifyUnconfirmed in that case, never VerifyValid, since nothing
// outside the document itself was checked.
func Verify(ctx context.Context, client dynamic.Interface, a AIBOM) VerifyResult {
	if a.Signature == "" || a.SignaturePublicKey == "" {
		return VerifyResult{Status: VerifyUnsigned, Detail: "no signature present"}
	}

	sig, err := base64.StdEncoding.DecodeString(a.Signature)
	if err != nil {
		return VerifyResult{Status: VerifyInvalid, Detail: fmt.Sprintf("signature is not valid base64: %v", err)}
	}
	pubKeyBytes, err := base64.StdEncoding.DecodeString(a.SignaturePublicKey)
	if err != nil {
		return VerifyResult{Status: VerifyInvalid, Detail: fmt.Sprintf("signaturePublicKey is not valid base64: %v", err)}
	}
	if len(pubKeyBytes) != ed25519.PublicKeySize {
		return VerifyResult{Status: VerifyInvalid, Detail: fmt.Sprintf("signaturePublicKey is %d bytes, want %d", len(pubKeyBytes), ed25519.PublicKeySize)}
	}

	canonical, err := canonicalizeData(a.RawData)
	if err != nil {
		return VerifyResult{Status: VerifyInvalid, Detail: fmt.Sprintf("could not canonicalize data: %v", err)}
	}

	if !ed25519.Verify(ed25519.PublicKey(pubKeyBytes), canonical, sig) {
		return VerifyResult{Status: VerifyInvalid, Detail: "signature does not match spec.data under the embedded public key -- data was altered after signing, or the signature/key is corrupt"}
	}

	if client == nil || a.Namespace == "" {
		return VerifyResult{Status: VerifyUnconfirmed, Detail: "signature is internally consistent, but no cluster was available to cross-check the signing key"}
	}

	clusterKey, err := fetchClusterPublicKey(ctx, client, a.Namespace)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return VerifyResult{Status: VerifyUnconfirmed, Detail: fmt.Sprintf("cluster has no %s ConfigMap yet in namespace %s", signingPublicKeyConfigMapName, a.Namespace)}
		}
		return VerifyResult{Status: VerifyUnconfirmed, Detail: fmt.Sprintf("could not reach cluster's signing-key anchor: %v", err)}
	}
	if clusterKey != a.SignaturePublicKey {
		return VerifyResult{Status: VerifyKeyMismatch, Detail: "signature is valid, but the embedded public key does not match the cluster's published key -- possible key rotation, or a forged signature paired with its own key"}
	}

	return VerifyResult{Status: VerifyValid, Detail: "signature matches spec.data, and the signing key matches the cluster's published anchor"}
}

// canonicalizeData produces the same RFC 8785 (JCS) canonical bytes that
// aibom-webhook-service's postprocess.py signed spec.data with (see its
// sign_aibom, and CLAUDE.md's Compiled AIBOM Signing). data is re-marshaled
// with the standard library first -- key order and whitespace don't matter,
// jcs.Transform re-parses and re-canonicalizes regardless -- so this only
// needs data to be valid JSON, not already canonical.
func canonicalizeData(data map[string]any) ([]byte, error) {
	raw, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("marshaling data: %w", err)
	}
	canonical, err := jcs.Transform(raw)
	if err != nil {
		return nil, fmt.Errorf("canonicalizing data: %w", err)
	}
	return canonical, nil
}

// fetchClusterPublicKey reads the live aibom-compiled-signing-public-key
// ConfigMap postprocess.py publishes (see CLAUDE.md's Compiled AIBOM
// Signing) via the same dynamic client this package already uses for AIBOM
// objects, rather than requiring a second, typed clientset just for this.
func fetchClusterPublicKey(ctx context.Context, client dynamic.Interface, namespace string) (string, error) {
	obj, err := client.Resource(configMapGVR).Namespace(namespace).Get(ctx, signingPublicKeyConfigMapName, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	value, _, err := unstructured.NestedString(obj.Object, "data", signingPublicKeyDataKey)
	if err != nil {
		return "", fmt.Errorf("reading data.%s: %w", signingPublicKeyDataKey, err)
	}
	return value, nil
}
