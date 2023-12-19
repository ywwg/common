// Copyright 2015 The Prometheus Authors
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

// Package expfmt contains tools for reading and writing Prometheus metrics.
package expfmt

import (
	"fmt"

	"github.com/prometheus/common/internal/bitbucket.org/ww/goautoneg"
	"github.com/prometheus/common/model"
)

// Format specifies the HTTP content type of the different wire protocols.
type Format string

// Constants to assemble the Content-Type values for the different wire
// protocols. The Content-Type strings here are all for the legacy exposition
// formats, where valid characters for metric names and label names are limited.
// Support for arbitrary UTF-8 characters in those names is already partially
// implemented in this module (see model.ValidationScheme), but to actually use
// it on the wire, new content-type strings will have to be agreed upon and
// added here.
const (
	TextVersion_1_0_0        = "1.0.0"
	TextVersion_0_0_4        = "0.0.4"
	ProtoType                = `application/vnd.google.protobuf`
	ProtoProtocol            = `io.prometheus.client.MetricFamily`
	ProtoFmt                 = ProtoType + "; proto=" + ProtoProtocol + ";"
	UTF8Valid                = "utf8"
	OpenMetricsType          = `application/openmetrics-text`
	OpenMetricsVersion_2_0_0 = "2.0.0"
	OpenMetricsVersion_1_0_0 = "1.0.0"
	OpenMetricsVersion_0_0_1 = "0.0.1"

	// The Content-Type values for the different wire protocols.
	FmtUnknown                Format = `<unknown>`
	FmtText_0_0_4             Format = `text/plain; version=` + TextVersion_0_0_4 + `; charset=utf-8`
	FmtText_1_0_0             Format = `text/plain; version=` + TextVersion_1_0_0 + `; charset=utf-8`
	FmtProtoDelim             Format = ProtoFmt + ` encoding=delimited`
	FmtProtoText              Format = ProtoFmt + ` encoding=text`
	FmtProtoCompact           Format = ProtoFmt + ` encoding=compact-text`
	FmtOpenMetrics_0_0_1      Format = OpenMetricsType + `; version=` + OpenMetricsVersion_0_0_1 + `; charset=utf-8`
	FmtOpenMetrics_1_0_0      Format = OpenMetricsType + `; version=` + OpenMetricsVersion_1_0_0 + `; charset=utf-8`
	FmtOpenMetrics_2_0_0      Format = OpenMetricsType + `; version=` + OpenMetricsVersion_2_0_0 + `; charset=utf-8`

	// UTF8 and Escaping Formats
	FmtUTF8Param         Format = `; validchars=utf8`
	FmtEscapeNone        Format = "none"
	FmtEscapeUnderscores Format = "underscores"
	FmtEscapeDots        Format = "dots"
	FmtEscapeValues      Format = "values"
)

const (
	hdrContentType = "Content-Type"
	hdrAccept      = "Accept"
)

func EscapingSchemeToFormat(s model.EscapingScheme) Format {
	switch s {
		case model.NoEscaping:
			return FmtEscapeNone
		case model.UnderscoreEscaping:
			return FmtEscapeUnderscores
		case model.DotsEscaping:
			return FmtEscapeDots
		case model.ValueEncodingEscaping:
			return FmtEscapeValues
		default:
			panic(fmt.Sprintf("unknown escaping scheme %d", s))
	}
}

func FormatToEscapingScheme(format Format) model.EscapingScheme {
	// XXXXXXXXXXXX this should be ParseContentType, not ParseAccept -- however
	// the basic parsing algo is probably fine? and then we can have a more
	// intelligent way of matching format than the string comparisons.

	// Probably, Format needs to be a proper class with matcher functions rather
	// than this thing we've got. Naturally people use the old strings everywhere
	// but I don't think that's ok.
	for _, ac := range goautoneg.ParseAccept(string(format)) {
		if escapeParam := ac.Params["escaping"]; escapeParam != "" {
			switch Format(escapeParam) {
				case FmtEscapeNone:
					return model.NoEscaping
				case FmtEscapeUnderscores:
					return model.UnderscoreEscaping
				case FmtEscapeDots :
					return model.DotsEscaping
				case FmtEscapeValues:
					return model.ValueEncodingEscaping
				default:
					panic("unknown format scheme "+escapeParam)
			}
		}
	}
	return model.NoEscaping
}