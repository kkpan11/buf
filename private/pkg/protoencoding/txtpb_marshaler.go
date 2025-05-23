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

package protoencoding

import (
	"fmt"

	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/proto"
)

type txtpbMarshaler struct {
	resolver Resolver
}

func newTxtpbMarshaler(resolver Resolver) Marshaler {
	if resolver == nil {
		resolver = EmptyResolver
	}
	return &txtpbMarshaler{
		resolver: resolver,
	}
}

func (m *txtpbMarshaler) Marshal(message proto.Message) ([]byte, error) {
	options := prototext.MarshalOptions{
		Resolver: m.resolver,
		Indent:   "  ",
	}
	data, err := options.Marshal(message)
	if err != nil {
		return nil, fmt.Errorf("txtpb marshal: %w", err)
	}
	return data, err
}
