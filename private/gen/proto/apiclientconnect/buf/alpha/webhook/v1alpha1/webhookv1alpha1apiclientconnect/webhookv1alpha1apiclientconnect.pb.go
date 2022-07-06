// Copyright 2020-2022 Buf Technologies, Inc.
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

// Code generated by protoc-gen-go-apiclientconnect. DO NOT EDIT.

package webhookv1alpha1apiclientconnect

import (
	context "context"
	webhookv1alpha1api "github.com/bufbuild/buf/private/gen/proto/api/buf/alpha/webhook/v1alpha1/webhookv1alpha1api"
	webhookv1alpha1apiclient "github.com/bufbuild/buf/private/gen/proto/apiclient/buf/alpha/webhook/v1alpha1/webhookv1alpha1apiclient"
	webhookv1alpha1connect "github.com/bufbuild/buf/private/gen/proto/connect/buf/alpha/webhook/v1alpha1/webhookv1alpha1connect"
	connect_go "github.com/bufbuild/connect-go"
	zap "go.uber.org/zap"
)

// NewProvider returns a new Provider.
func NewProvider(
	logger *zap.Logger,
	httpClient connect_go.HTTPClient,
	options ...ProviderOption,
) webhookv1alpha1apiclient.Provider {
	provider := &provider{
		logger:     logger,
		httpClient: httpClient,
	}
	for _, option := range options {
		option(provider)
	}
	return provider
}

type provider struct {
	logger                  *zap.Logger
	httpClient              connect_go.HTTPClient
	addressMapper           func(string) string
	contextModifierProvider func(string) (func(context.Context) context.Context, error)
}

// ProviderOption is an option for a new Provider.
type ProviderOption func(*provider)

// WithAddressMapper maps the address with the given function.
func WithAddressMapper(addressMapper func(string) string) ProviderOption {
	return func(provider *provider) {
		provider.addressMapper = addressMapper
	}
}

// WithContextModifierProvider provides a function that  modifies the context before every RPC invocation.
// Applied before the address mapper.
func WithContextModifierProvider(contextModifierProvider func(address string) (func(context.Context) context.Context, error)) ProviderOption {
	return func(provider *provider) {
		provider.contextModifierProvider = contextModifierProvider
	}
}

func (p *provider) NewEventService(ctx context.Context, address string) (webhookv1alpha1api.EventService, error) {
	var contextModifier func(context.Context) context.Context
	var err error
	if p.contextModifierProvider != nil {
		contextModifier, err = p.contextModifierProvider(address)
		if err != nil {
			return nil, err
		}
	}
	if p.addressMapper != nil {
		address = p.addressMapper(address)
	}
	return &eventServiceClient{
		logger: p.logger,
		client: webhookv1alpha1connect.NewEventServiceClient(
			p.httpClient,
			address,
		),
		contextModifier: contextModifier,
	}, nil
}