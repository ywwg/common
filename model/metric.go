// Copyright 2013 The Prometheus Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package model

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"

	dto "github.com/prometheus/client_model/go"
	"google.golang.org/protobuf/proto"
)

// ValidationScheme is a Go enum for determining how metric and label names will
// be validated by this library.
type ValidationScheme int

const (
	// LegacyValidation is a setting that requirets that metric and label names
	// conform to the original Prometheus character requirements described by
	// MetricNameRE and LabelNameRE.
	LegacyValidation ValidationScheme = iota

	// UTF8Validation only requires that metric and label names be valid UTF8
	// strings.
	UTF8Validation
)

type EscapingScheme int

const (
	// NoEscaping indicates that a name will not be escaped.
	NoEscaping EscapingScheme = iota

	// UnderscoreEscaping replaces all legacy-invalid characters with underscores.
	UnderscoreEscaping

	// DotsEscaping is similar to UnderscoreEscaping, except that dots are
	// converted to `_dot_` and pre-existing underscores are converted to `__`.
	DotsEscaping

	// ValueEncodingEscaping prepends the name with `U__` and replaces all invalid
	// characters with the unicode value, surrounded by underscores. Single
	// underscores are replaced with double underscores.
	ValueEncodingEscaping
)

var (
	// NameValidationScheme determines the method of name validation to be used by
	// all calls to IsValidMetricName() and LabelName IsValid(). Setting UTF8 mode
	// in isolation from other components that don't support UTF8 may result in
	// bugs or other undefined behavior. This value is intended to be set by
	// UTF8-aware binaries as part of their startup. To avoid need for locking,
	// this value should be set once, ideally in an init(), before multiple
	// goroutines are started.
	NameValidationScheme = LegacyValidation

	// MetricNameRE is a regular expression matching valid metric
	// names. Note that the IsValidMetricName function performs the same
	// check but faster than a match with this regular expression.
	MetricNameRE = regexp.MustCompile(`^[a-zA-Z_:][a-zA-Z0-9_:]*$`)
	// NameValidationScheme determines the default method of name validation to be
	// used. To avoid need for locking, this value should be set once, probably in
	// an init(), before multiple goroutines are started.
	NameValidationScheme = LegacyValidation

	// DefaultNameEscapingScheme defines the default way that names will be
	// escaped when presented to systems that do not support UTF-8 names.
	DefaultNameEscapingScheme = UnderscoreEscaping
)

// A Metric is similar to a LabelSet, but the key difference is that a Metric is
// a singleton and refers to one and only one stream of samples.
type Metric LabelSet

// Equal compares the metrics.
func (m Metric) Equal(o Metric) bool {
	return LabelSet(m).Equal(LabelSet(o))
}

// Before compares the metrics' underlying label sets.
func (m Metric) Before(o Metric) bool {
	return LabelSet(m).Before(LabelSet(o))
}

// Clone returns a copy of the Metric.
func (m Metric) Clone() Metric {
	clone := make(Metric, len(m))
	for k, v := range m {
		clone[k] = v
	}
	return clone
}

func (m Metric) String() string {
	metricName, hasName := m[MetricNameLabel]
	numLabels := len(m) - 1
	if !hasName {
		numLabels = len(m)
	}
	labelStrings := make([]string, 0, numLabels)
	for label, value := range m {
		if label != MetricNameLabel {
			labelStrings = append(labelStrings, fmt.Sprintf("%s=%q", label, value))
		}
	}

	switch numLabels {
	case 0:
		if hasName {
			return string(metricName)
		}
		return "{}"
	default:
		sort.Strings(labelStrings)
		return fmt.Sprintf("%s{%s}", metricName, strings.Join(labelStrings, ", "))
	}
}

// Fingerprint returns a Metric's Fingerprint.
func (m Metric) Fingerprint() Fingerprint {
	return LabelSet(m).Fingerprint()
}

// FastFingerprint returns a Metric's Fingerprint calculated by a faster hashing
// algorithm, which is, however, more susceptible to hash collisions.
func (m Metric) FastFingerprint() Fingerprint {
	return LabelSet(m).FastFingerprint()
}

// IsValidMetricName returns true iff name matches the pattern of MetricNameRE
// for legacy names, and iff it's valid UTF-8 if the UTF8Validation scheme is
// selected.
func IsValidMetricName(n LabelValue) bool {
	switch NameValidationScheme {
	case LegacyValidation:
		return IsValidLegacyMetricName(n)
	case UTF8Validation:
		if len(n) == 0 {
			return false
		}
		return utf8.ValidString(string(n))
	default:
		panic(fmt.Sprintf("Invalid name validation scheme requested: %d", NameValidationScheme))
	}
}

// IsValidLegacyMetricName is similar to IsValidMetricName but always uses the
// legacy validation scheme regardless of the value of NameValidationScheme.
// This function, however, does not use MetricNameRE for the check but a much
// faster hardcoded implementation.
func IsValidLegacyMetricName(n LabelValue) bool {
	if len(n) == 0 {
		return false
	}
	for i, b := range n {
		if !isLegacyValidRune(b, i) {
			return false
		}
	}
	return true
}

// EscapeMetricFamily escapes the given metric names and labels in place with
// the given escaping scheme. Uses the same pointers to fields when possible,
// otherwise creates new escaped versions so as not to mutate the input.
func EscapeMetricFamily(v *dto.MetricFamily, scheme EscapingScheme) *dto.MetricFamily {
	if v == nil {
		return nil
	}

	if scheme == NoEscaping {
		return v
	}

	out := &dto.MetricFamily{
		Help: v.Help,
		Type: v.Type,
	}

	// If the name is nil, copy as-is, don't try to escape.
	if v.Name == nil || IsValidLegacyMetricName(LabelValue(v.GetName())) {
		out.Name = v.Name
	} else {
		out.Name = proto.String(EscapeName(v.GetName(), scheme))
	}
	for _, m := range v.Metric {
		if !metricNeedsEscaping(m) {
			out.Metric = append(out.Metric, m)
			continue
		}

		escaped := &dto.Metric{
			Gauge:       m.Gauge,
			Counter:     m.Counter,
			Summary:     m.Summary,
			Untyped:     m.Untyped,
			Histogram:   m.Histogram,
			TimestampMs: m.TimestampMs,
		}

		for _, l := range m.Label {
			if l.GetName() == MetricNameLabel {
				if l.Value == nil || IsValidLegacyMetricName(LabelValue(l.GetValue())) {
					escaped.Label = append(escaped.Label, l)
					continue
				}
				escaped.Label = append(escaped.Label, &dto.LabelPair{
					Name:  proto.String(MetricNameLabel),
					Value: proto.String(EscapeName(l.GetValue(), scheme)),
				})
				continue
			}
			if l.Name == nil || IsValidLegacyMetricName(LabelValue(l.GetName())) {
				escaped.Label = append(escaped.Label, l)
				continue
			}
			escaped.Label = append(escaped.Label, &dto.LabelPair{
				Name:  proto.String(EscapeName(l.GetName(), scheme)),
				Value: l.Value,
			})
		}
		out.Metric = append(out.Metric, escaped)
	}
	return out
}

func metricNeedsEscaping(m *dto.Metric) bool {
	for _, l := range m.Label {
		if *l.Name == MetricNameLabel && !IsValidLegacyMetricName(LabelValue(*l.Value)) {
			return true
		}
		if !IsValidLegacyMetricName(LabelValue(*l.Value)) {
			return true
		}
	}
	return false
}

const (
	lowerhex = "0123456789abcdef"
)

// EscapeName escapes the incoming name according to the provided escaping
// scheme. Depending on the rules of escaping, this may cause no change in the
// string that is returned. (Especially NoEscaping, which by definition is a noop).
// This function does not do any validation of the name.
func EscapeName(name string, scheme EscapingScheme) string {
	if len(name) == 0 {
		return name
	}
	var escaped strings.Builder
	switch scheme {
	case NoEscaping:
		return name
	case UnderscoreEscaping:
		if IsValidLegacyMetricName(LabelValue(name)) {
			return name
		}
		for i, b := range name {
			if isLegacyValidRune(b, i) {
				escaped.WriteRune(b)
			} else {
				escaped.WriteRune('_')
			}
		}
		return escaped.String()
	case DotsEscaping:
		// Do not early return for legacy valid names, we still escape underscores.
		for i, b := range name {
			if b == '_' {
				escaped.WriteString("__")
			} else if b == '.' {
				escaped.WriteString("_dot_")
			} else if isLegacyValidRune(b, i) {
				escaped.WriteRune(b)
			} else {
				escaped.WriteRune('_')
			}
		}
		return escaped.String()
	case ValueEncodingEscaping:
		if IsValidLegacyMetricName(LabelValue(name)) {
			return name
		}
		escaped.WriteString("U__")
		for i, b := range name {
			if isLegacyValidRune(b, i) {
				escaped.WriteRune(b)
			} else if !utf8.ValidRune(b) {
				escaped.WriteString("_FFFD_")
			} else if b < 0x100 {
				escaped.WriteRune('_')
				for s := 4; s >= 0; s -= 4 {
					escaped.WriteByte(lowerhex[b>>uint(s)&0xF])
				}
				escaped.WriteRune('_')
			} else if b < 0x10000 {
				escaped.WriteRune('_')
				for s := 12; s >= 0; s -= 4 {
					escaped.WriteByte(lowerhex[b>>uint(s)&0xF])
				}
				escaped.WriteRune('_')
			}
		}
		return escaped.String()
	default:
		panic(fmt.Sprintf("invalid escaping scheme %d", scheme))
	}
}

func isLegacyValidRune(b rune, i int) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || b == '_' || b == ':' || (b >= '0' && b <= '9' && i > 0)
}
