package aibom

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

// SeriesStatus is the outcome of checking an AIBOM's persisted telemetry
// time series against the digest in its signed data. See
// aibom-webhook-service's CLAUDE.md, "Telemetry Time Series".
type SeriesStatus int

const (
	// SeriesAbsent means the AIBOM carries no telemetry_series_ref at all --
	// created before the feature shipped, or Prometheus/the cap/RBAC meant no
	// series was stored. Not a bad sign; callers should print nothing.
	SeriesAbsent SeriesStatus = iota

	// SeriesVerified means the AIBOMTelemetry object's spec.seriesJson hashes
	// to the sha256 in spec.data.telemetry_series_ref, and the AIBOM's own
	// signature is Valid -- so that digest is itself authenticated. The only
	// status that should be presented as "trust this."
	SeriesVerified

	// SeriesDigestOnly means the content matches the digest in the AIBOM, but
	// the AIBOM's signature isn't Valid (unsigned, unconfirmed key, ...). The
	// digest sits inside spec.data, so without a trustworthy signature over
	// it, whoever could forge the series could forge the digest too -- this
	// only proves the object wasn't changed independently of the AIBOM.
	SeriesDigestOnly

	// SeriesMismatch means the object exists but its content does not match
	// the signed digest (or the recorded size) -- it was altered or replaced
	// after the AIBOM was created.
	SeriesMismatch

	// SeriesMissing means the AIBOM references an object that doesn't exist
	// (deleted, or the create never landed). Not evidence of tampering by
	// itself, but the recorded series is gone.
	SeriesMissing

	// SeriesUnconfirmed means the object couldn't be checked: no cluster to
	// read it from (archived export), no RBAC, a network error, or a
	// reference shape this version doesn't understand.
	SeriesUnconfirmed
)

// SeriesResult is the outcome of VerifySeries, along with a human-readable
// Detail explaining why.
type SeriesResult struct {
	Status SeriesStatus
	Detail string
}

// TelemetryGVR identifies the AIBOMTelemetry custom resource that holds an
// AIBOM's downsampled telemetry series.
var TelemetryGVR = schema.GroupVersionResource{
	Group:    "aibom.io",
	Version:  "v1alpha1",
	Resource: "aibomtelemetries",
}

const telemetryKind = "AIBOMTelemetry"

// VerifySeries checks a's persisted telemetry series against the digest in
// its signed spec.data.telemetry_series_ref: it reads the referenced
// AIBOMTelemetry object and hashes the exact spec.seriesJson string. It reads
// the reference from RawData -- the same generic tree Verify signs over --
// not the typed Data struct.
//
// aibomResult is Verify's outcome for the same AIBOM. The digest only
// authenticates the series if the signature over it is itself valid, so
// anything short of VerifyValid caps a matching series at SeriesDigestOnly.
// Pass a nil client (or an AIBOM with no namespace) to skip the cluster read,
// e.g. for an archived export; that yields SeriesUnconfirmed.
func VerifySeries(ctx context.Context, client dynamic.Interface, a AIBOM, aibomResult VerifyResult) SeriesResult {
	ref, ok := a.RawData["telemetry_series_ref"].(map[string]any)
	if !ok {
		return SeriesResult{Status: SeriesAbsent}
	}

	kind, _ := ref["kind"].(string)
	if kind != telemetryKind {
		// Includes the short-lived first form that stored the series in a
		// ConfigMap ({"configmap": ..., "key": ...}); those aren't checked here.
		return SeriesResult{Status: SeriesUnconfirmed, Detail: fmt.Sprintf("telemetry_series_ref has an unrecognized shape (kind %q); not checked", kind)}
	}
	name, _ := ref["name"].(string)
	wantSHA, _ := ref["sha256"].(string)
	if name == "" || wantSHA == "" {
		return SeriesResult{Status: SeriesUnconfirmed, Detail: "telemetry_series_ref is missing its name or sha256; not checked"}
	}

	if client == nil || a.Namespace == "" {
		return SeriesResult{Status: SeriesUnconfirmed, Detail: "no cluster was available to read the telemetry object"}
	}

	obj, err := client.Resource(TelemetryGVR).Namespace(a.Namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return SeriesResult{Status: SeriesMissing, Detail: fmt.Sprintf("%s/%s no longer exists in namespace %s", telemetryKind, name, a.Namespace)}
		}
		return SeriesResult{Status: SeriesUnconfirmed, Detail: fmt.Sprintf("could not read %s/%s: %v", telemetryKind, name, err)}
	}

	seriesJSON, found, err := unstructured.NestedString(obj.Object, "spec", "seriesJson")
	if err != nil || !found {
		return SeriesResult{Status: SeriesMismatch, Detail: fmt.Sprintf("%s/%s has no spec.seriesJson string", telemetryKind, name)}
	}

	sum := sha256.Sum256([]byte(seriesJSON))
	if gotSHA := hex.EncodeToString(sum[:]); !strings.EqualFold(gotSHA, wantSHA) {
		return SeriesResult{Status: SeriesMismatch, Detail: "spec.seriesJson does not match the sha256 in the AIBOM's signed data -- the series was altered or replaced after the AIBOM was created"}
	}
	if want, ok := asInt64(ref["size_bytes"]); ok && want != int64(len(seriesJSON)) {
		return SeriesResult{Status: SeriesMismatch, Detail: fmt.Sprintf("spec.seriesJson is %d bytes, but the AIBOM's signed data records %d", len(seriesJSON), want)}
	}

	if aibomResult.Status != VerifyValid {
		return SeriesResult{Status: SeriesDigestOnly, Detail: "content matches the digest in the AIBOM, but the AIBOM's own signature isn't verified, so the digest itself isn't authenticated"}
	}
	return SeriesResult{Status: SeriesVerified, Detail: "spec.seriesJson matches the digest in the AIBOM's verified signed data"}
}

// asInt64 reads a JSON number that may have decoded as int64 or float64.
func asInt64(v any) (int64, bool) {
	switch n := v.(type) {
	case int64:
		return n, true
	case int:
		return int64(n), true
	case float64:
		return int64(n), n == float64(int64(n))
	default:
		return 0, false
	}
}
