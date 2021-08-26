// Copyright 2020-2021 Buf Technologies, Inc.
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

// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.1.0
// - protoc             v3.17.3
// source: buf/alpha/registry/v1alpha1/recommendation.proto

package registryv1alpha1

import (
	context "context"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
// Requires gRPC-Go v1.32.0 or later.
const _ = grpc.SupportPackageIsVersion7

// RecommendationServiceClient is the client API for RecommendationService service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type RecommendationServiceClient interface {
	// RecommendedRepositories returns a list of recommended repositories.
	RecommendedRepositories(ctx context.Context, in *RecommendedRepositoriesRequest, opts ...grpc.CallOption) (*RecommendedRepositoriesResponse, error)
}

type recommendationServiceClient struct {
	cc grpc.ClientConnInterface
}

func NewRecommendationServiceClient(cc grpc.ClientConnInterface) RecommendationServiceClient {
	return &recommendationServiceClient{cc}
}

func (c *recommendationServiceClient) RecommendedRepositories(ctx context.Context, in *RecommendedRepositoriesRequest, opts ...grpc.CallOption) (*RecommendedRepositoriesResponse, error) {
	out := new(RecommendedRepositoriesResponse)
	err := c.cc.Invoke(ctx, "/buf.alpha.registry.v1alpha1.RecommendationService/RecommendedRepositories", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// RecommendationServiceServer is the server API for RecommendationService service.
// All implementations should embed UnimplementedRecommendationServiceServer
// for forward compatibility
type RecommendationServiceServer interface {
	// RecommendedRepositories returns a list of recommended repositories.
	RecommendedRepositories(context.Context, *RecommendedRepositoriesRequest) (*RecommendedRepositoriesResponse, error)
}

// UnimplementedRecommendationServiceServer should be embedded to have forward compatible implementations.
type UnimplementedRecommendationServiceServer struct {
}

func (UnimplementedRecommendationServiceServer) RecommendedRepositories(context.Context, *RecommendedRepositoriesRequest) (*RecommendedRepositoriesResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method RecommendedRepositories not implemented")
}

// UnsafeRecommendationServiceServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to RecommendationServiceServer will
// result in compilation errors.
type UnsafeRecommendationServiceServer interface {
	mustEmbedUnimplementedRecommendationServiceServer()
}

func RegisterRecommendationServiceServer(s grpc.ServiceRegistrar, srv RecommendationServiceServer) {
	s.RegisterService(&RecommendationService_ServiceDesc, srv)
}

func _RecommendationService_RecommendedRepositories_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(RecommendedRepositoriesRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(RecommendationServiceServer).RecommendedRepositories(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/buf.alpha.registry.v1alpha1.RecommendationService/RecommendedRepositories",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(RecommendationServiceServer).RecommendedRepositories(ctx, req.(*RecommendedRepositoriesRequest))
	}
	return interceptor(ctx, in, info, handler)
}

// RecommendationService_ServiceDesc is the grpc.ServiceDesc for RecommendationService service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var RecommendationService_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "buf.alpha.registry.v1alpha1.RecommendationService",
	HandlerType: (*RecommendationServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "RecommendedRepositories",
			Handler:    _RecommendationService_RecommendedRepositories_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "buf/alpha/registry/v1alpha1/recommendation.proto",
}