// Copyright 2020-2025 Buf Technologies, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package bufcheckserverutil

import (
	"buf.build/go/bufplugin/check"
	"github.com/bufbuild/buf/private/bufpkg/bufprotosource"
)

// ResponseWriter is a check.ResponseWriter that also includes bufprotosource functionality.
type ResponseWriter interface {
	check.ResponseWriter

	// AddProtosourceAnnotation adds a [check.Annotation] for the specified [bufprotosource.Location]s.
	//
	// If location is non-nil, it is used to populate the annotation's source code information,
	// and inputFileName is ignored. If location is nil and inputFileName is provided, the
	// latter is used as a fallback to set the input file name in the annotation.
	// againstLocation is optional and may be nil.
	//
	// The format string and args are used to construct the annotation message.
	AddProtosourceAnnotation(
		location bufprotosource.Location,
		againstLocation bufprotosource.Location,
		inputFileName string,
		format string,
		args ...any,
	)
}

type responseWriter struct {
	check.ResponseWriter
}

func newResponseWriter(checkResponseWriter check.ResponseWriter) *responseWriter {
	return &responseWriter{
		ResponseWriter: checkResponseWriter,
	}
}

func (w *responseWriter) AddProtosourceAnnotation(
	location bufprotosource.Location,
	againstLocation bufprotosource.Location,
	inputFileName string,
	format string,
	args ...any,
) {
	addAnnotationOptions := []check.AddAnnotationOption{
		check.WithMessagef(format, args...),
	}
	if location != nil {
		addAnnotationOptions = append(
			addAnnotationOptions,
			check.WithFileNameAndSourcePath(location.FilePath(), location.SourcePath()),
		)
	} else if inputFileName != "" {
		addAnnotationOptions = append(
			addAnnotationOptions,
			check.WithFileName(inputFileName),
		)
	}
	if againstLocation != nil {
		addAnnotationOptions = append(
			addAnnotationOptions,
			check.WithAgainstFileNameAndSourcePath(againstLocation.FilePath(), againstLocation.SourcePath()),
		)
	}
	w.ResponseWriter.AddAnnotation(addAnnotationOptions...)
}
