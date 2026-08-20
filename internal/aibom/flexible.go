package aibom

import (
	"encoding/json"
	"strconv"
	"strings"
)

// spec.data is x-kubernetes-preserve-unknown-fields with no schema, and
// several of its numeric fields are populated from shell command output or
// best-effort CLI-arg parsing that falls back to the raw string on a failed
// int()/float() conversion (see postprocess.py's detect_trl_from_command /
// detect_vllm_from_command). FlexInt/FlexFloat tolerate a JSON string in
// place of a number instead of failing to decode the whole AIBOM.

// FlexInt unmarshals from either a JSON number or a numeric-looking JSON
// string. Malformed/non-numeric strings decode to zero rather than erroring,
// since this is best-effort display data, not something to fail a `list` on.
type FlexInt int

func (f *FlexInt) UnmarshalJSON(b []byte) error {
	var n int
	if err := json.Unmarshal(b, &n); err == nil {
		*f = FlexInt(n)
		return nil
	}
	var s string
	if err := json.Unmarshal(b, &s); err == nil {
		s = strings.TrimSpace(s)
		if v, err := strconv.Atoi(s); err == nil {
			*f = FlexInt(v)
			return nil
		}
		if v, err := strconv.ParseFloat(s, 64); err == nil {
			*f = FlexInt(int(v))
			return nil
		}
	}
	return nil
}

// FlexFloat unmarshals from either a JSON number or a numeric-looking JSON
// string, for the same reason as FlexInt.
type FlexFloat float64

func (f *FlexFloat) UnmarshalJSON(b []byte) error {
	var n float64
	if err := json.Unmarshal(b, &n); err == nil {
		*f = FlexFloat(n)
		return nil
	}
	var s string
	if err := json.Unmarshal(b, &s); err == nil {
		if v, err := strconv.ParseFloat(strings.TrimSpace(s), 64); err == nil {
			*f = FlexFloat(v)
			return nil
		}
	}
	return nil
}
