// Copyright 2018 The Prometheus Authors
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

package expfmt

import (
	"bytes"
	"net/http"
	"testing"

	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/model"
	"google.golang.org/protobuf/proto"
)

func TestNegotiate(t *testing.T) {
	acceptValuePrefix := "application/vnd.google.protobuf;proto=io.prometheus.client.MetricFamily"
	tests := []struct {
		name              string
		acceptHeaderValue string
		expectedFmt       string
	}{
		{
			name:              "delimited format",
			acceptHeaderValue: acceptValuePrefix + ";encoding=delimited",
			expectedFmt:       "application/vnd.google.protobuf; proto=io.prometheus.client.MetricFamily; encoding=delimited; escaping=underscores",
		},
		{
			name:              "text format",
			acceptHeaderValue: acceptValuePrefix + ";encoding=text",
			expectedFmt:       "application/vnd.google.protobuf; proto=io.prometheus.client.MetricFamily; encoding=text; escaping=underscores",
		},
		{
			name:              "compact text format",
			acceptHeaderValue: acceptValuePrefix + ";encoding=compact-text",
			expectedFmt:       "application/vnd.google.protobuf; proto=io.prometheus.client.MetricFamily; encoding=compact-text; escaping=underscores",
		},
		{
			name:              "plain text format",
			acceptHeaderValue: "text/plain;version=0.0.4",
			expectedFmt:       "text/plain; version=0.0.4; charset=utf-8; escaping=underscores",
		},
		{
			name:              "delimited format utf8",
			acceptHeaderValue: acceptValuePrefix + ";encoding=delimited; validation-scheme=utf8;",
			expectedFmt:       "application/vnd.google.protobuf; proto=io.prometheus.client.MetricFamily; encoding=delimited; validchars=utf8",
		},
		{
			name:              "text format utf8",
			acceptHeaderValue: acceptValuePrefix + ";encoding=text; validation-scheme=utf8;",
			expectedFmt:       "application/vnd.google.protobuf; proto=io.prometheus.client.MetricFamily; encoding=text; validchars=utf8",
		},
		{
			name:              "compact text format utf8",
			acceptHeaderValue: acceptValuePrefix + ";encoding=compact-text; validation-scheme=utf8;",
			expectedFmt:       "application/vnd.google.protobuf; proto=io.prometheus.client.MetricFamily; encoding=compact-text; validchars=utf8",
		},
		{
			name:              "plain text format 0.0.4 with utf8 not valid, falls back",
			acceptHeaderValue: "text/plain;version=0.0.4;validation-scheme=utf8;",
			expectedFmt:       "text/plain; version=0.0.4; charset=utf-8; escaping=underscores",
		},
		{
			name:              "plain text format 1.0.0",
			acceptHeaderValue: "text/plain;version=1.0.0;",
			expectedFmt:       "text/plain; version=1.0.0; charset=utf-8; escaping=underscores",
		},
		{
			name:              "plain text format 1.0.0 with utf8",
			acceptHeaderValue: "text/plain;version=1.0.0; validation-scheme=utf8;",
			expectedFmt:       "text/plain; version=1.0.0; charset=utf-8; validchars=utf8",
		},
	}

	oldDefault := model.DefaultNameEscapingScheme
	model.DefaultNameEscapingScheme = model.UnderscoreEscaping
	defer func() {
		model.DefaultNameEscapingScheme = oldDefault
	}()

	for i, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			h := http.Header{}
			h.Add(hdrAccept, test.acceptHeaderValue)
			actualFmt := string(Negotiate(h))
			if actualFmt != test.expectedFmt {
				t.Errorf("case %d: expected Negotiate to return format %s, but got %s instead", i, test.expectedFmt, actualFmt)
			}
		})
	}
}

func TestNegotiateOpenMetrics(t *testing.T) {
	tests := []struct {
		name              string
		acceptHeaderValue string
		expectedFmt       string
	}{
		{
			name:              "OM format, no version",
			acceptHeaderValue: "application/openmetrics-text",
			expectedFmt:       "application/openmetrics-text; version=0.0.1; charset=utf-8; escaping=values",
		},
		{
			name:              "OM format, 0.0.1 version",
			acceptHeaderValue: "application/openmetrics-text;version=0.0.1",
			expectedFmt:       "application/openmetrics-text; version=0.0.1; charset=utf-8; escaping=values",
		},
		{
			name:              "OM format, 1.0.0 version",
			acceptHeaderValue: "application/openmetrics-text;version=1.0.0",
			expectedFmt:       "application/openmetrics-text; version=1.0.0; charset=utf-8; escaping=values",
		},
		{
			name:              "OM format, 2.0.0 version, legacy",
			acceptHeaderValue: "application/openmetrics-text;version=2.0.0;escaping=dots",
			expectedFmt:       "application/openmetrics-text; version=2.0.0; charset=utf-8; escaping=dots",
		},
		{
			name:              "OM format, 2.0.0 version, utf8",
			acceptHeaderValue: "application/openmetrics-text;version=2.0.0;validation-scheme=utf8;",
			expectedFmt:       "application/openmetrics-text; version=2.0.0; charset=utf-8; validchars=utf8",
		},
		{
			name:              "OM format, 0.0.1 version with utf8 is not valid, falls back",
			acceptHeaderValue: "application/openmetrics-text;version=0.0.1;validation-scheme=utf8;",
			expectedFmt:       "application/openmetrics-text; version=0.0.1; charset=utf-8; escaping=values",
		},
		{
			name:              "OM format, 1.0.0 version with utf8 is not valid, falls back",
			acceptHeaderValue: "application/openmetrics-text;version=1.0.0;validation-scheme=utf8;",
			expectedFmt:       "application/openmetrics-text; version=1.0.0; charset=utf-8; escaping=values",
		},
		{
			name:              "OM format, invalid version",
			acceptHeaderValue: "application/openmetrics-text;version=0.0.4",
			expectedFmt:       "text/plain; version=0.0.4; charset=utf-8; escaping=values",
		},
	}

	oldDefault := model.DefaultNameEscapingScheme
	model.DefaultNameEscapingScheme = model.ValueEncodingEscaping
	defer func() {
		model.DefaultNameEscapingScheme = oldDefault
	}()

	for i, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			h := http.Header{}
			h.Add(hdrAccept, test.acceptHeaderValue)
			actualFmt := string(NegotiateIncludingOpenMetrics(h))
			if actualFmt != test.expectedFmt {
				t.Errorf("case %d: expected Negotiate to return format %s, but got %s instead", i, test.expectedFmt, actualFmt)
			}
		})
	}
}

func TestEncode(t *testing.T) {
	var buff bytes.Buffer
	delimEncoder := NewEncoder(&buff, FmtProtoDelim)
	metric := &dto.MetricFamily{
		Name: proto.String("foo_metric"),
		Type: dto.MetricType_UNTYPED.Enum(),
		Metric: []*dto.Metric{
			{
				Untyped: &dto.Untyped{
					Value: proto.Float64(1.234),
				},
			},
		},
	}

	err := delimEncoder.Encode(metric)
	if err != nil {
		t.Errorf("unexpected error during encode: %s", err.Error())
	}

	out := buff.Bytes()
	if len(out) == 0 {
		t.Errorf("expected the output bytes buffer to be non-empty")
	}

	buff.Reset()

	compactEncoder := NewEncoder(&buff, FmtProtoCompact)
	err = compactEncoder.Encode(metric)
	if err != nil {
		t.Errorf("unexpected error during encode: %s", err.Error())
	}

	out = buff.Bytes()
	if len(out) == 0 {
		t.Errorf("expected the output bytes buffer to be non-empty")
	}

	buff.Reset()

	protoTextEncoder := NewEncoder(&buff, FmtProtoText)
	err = protoTextEncoder.Encode(metric)
	if err != nil {
		t.Errorf("unexpected error during encode: %s", err.Error())
	}

	out = buff.Bytes()
	if len(out) == 0 {
		t.Errorf("expected the output bytes buffer to be non-empty")
	}

	buff.Reset()

	textEncoder := NewEncoder(&buff, FmtText_0_0_4)
	err = textEncoder.Encode(metric)
	if err != nil {
		t.Errorf("unexpected error during encode: %s", err.Error())
	}

	out = buff.Bytes()
	if len(out) == 0 {
		t.Errorf("expected the output bytes buffer to be non-empty")
	}

	expected := "# TYPE foo_metric untyped\n" +
		"foo_metric 1.234\n"

	if string(out) != expected {
		t.Errorf("expected TextEncoder to return %s, but got %s instead", expected, string(out))
	}
}


func TestEscapedEncode(t *testing.T) {
	var buff bytes.Buffer
	delimEncoder := NewEncoder(&buff, FmtProtoDelim+"; escaping=underscores")
	metric := &dto.MetricFamily{
		Name: proto.String("foo.metric"),
		Type: dto.MetricType_UNTYPED.Enum(),
		Metric: []*dto.Metric{
			{
				Untyped: &dto.Untyped{
					Value: proto.Float64(1.234),
				},
			},
			{
				Label: []*dto.LabelPair{
					{
						Name: proto.String("dotted.label.name"),
						Value: proto.String("my.label.value"),
					},
				},
				Counter: &dto.Counter{
					Value: proto.Float64(8),
				},
			},
		},
	}

	err := delimEncoder.Encode(metric)
	if err != nil {
		t.Errorf("unexpected error during encode: %s", err.Error())
	}

	out := buff.Bytes()
	if len(out) == 0 {
		t.Errorf("expected the output bytes buffer to be non-empty")
	}

	buff.Reset()

	// compactEncoder := NewEncoder(&buff, FmtProtoCompact)
	// err = compactEncoder.Encode(metric)
	// if err != nil {
	// 	t.Errorf("unexpected error during encode: %s", err.Error())
	// }

	// out = buff.Bytes()
	// if len(out) == 0 {
	// 	t.Errorf("expected the output bytes buffer to be non-empty")
	// }

	// buff.Reset()

	// protoTextEncoder := NewEncoder(&buff, FmtProtoText)
	// err = protoTextEncoder.Encode(metric)
	// if err != nil {
	// 	t.Errorf("unexpected error during encode: %s", err.Error())
	// }

	// out = buff.Bytes()
	// if len(out) == 0 {
	// 	t.Errorf("expected the output bytes buffer to be non-empty")
	// }

	// buff.Reset()

	// textEncoder := NewEncoder(&buff, FmtText_0_0_4)
	// err = textEncoder.Encode(metric)
	// if err != nil {
	// 	t.Errorf("unexpected error during encode: %s", err.Error())
	// }

	// out = buff.Bytes()
	// if len(out) == 0 {
	// 	t.Errorf("expected the output bytes buffer to be non-empty")
	// }

	// expected := "# TYPE foo_metric untyped\n" +
	// 	"foo_metric 1.234\n"

	// if string(out) != expected {
	// 	t.Errorf("expected TextEncoder to return %s, but got %s instead", expected, string(out))
	// }
}