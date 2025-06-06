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

package bufmoduleapi

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	modulev1 "buf.build/gen/go/bufbuild/registry/protocolbuffers/go/buf/registry/module/v1"
	modulev1beta1 "buf.build/gen/go/bufbuild/registry/protocolbuffers/go/buf/registry/module/v1beta1"
	ownerv1 "buf.build/gen/go/bufbuild/registry/protocolbuffers/go/buf/registry/owner/v1"
	"connectrpc.com/connect"
	"github.com/bufbuild/buf/private/bufpkg/bufmodule"
	"github.com/bufbuild/buf/private/bufpkg/bufregistryapi/bufregistryapimodule"
	"github.com/bufbuild/buf/private/pkg/standard/xslices"
	"github.com/bufbuild/buf/private/pkg/syserror"
	"github.com/bufbuild/buf/private/pkg/uuidutil"
	"github.com/google/uuid"
)

// NewUploader returns a new Uploader for the given API client.
func NewUploader(
	logger *slog.Logger,
	moduleClientProvider interface {
		bufregistryapimodule.V1ModuleServiceClientProvider
		bufregistryapimodule.V1UploadServiceClientProvider
		bufregistryapimodule.V1Beta1UploadServiceClientProvider
	},
	options ...UploaderOption,
) bufmodule.Uploader {
	return newUploader(logger, moduleClientProvider, options...)
}

// UploaderOption is an option for a new Uploader.
type UploaderOption func(*uploader)

// UploaderWithPublicRegistry returns a new UploaderOption that specifies
// the hostname of the public registry. By default this is "buf.build", however in testing,
// this may be something else. This is needed to discern which which registry to make calls
// against in the case where there is >1 registries represented in the ModuleKeys - we always
// want to call the non-public registry.
func UploaderWithPublicRegistry(publicRegistry string) UploaderOption {
	return func(uploader *uploader) {
		if publicRegistry != "" {
			uploader.publicRegistry = publicRegistry
		}
	}
}

// *** PRIVATE ***

type uploader struct {
	logger               *slog.Logger
	moduleClientProvider interface {
		bufregistryapimodule.V1ModuleServiceClientProvider
		bufregistryapimodule.V1UploadServiceClientProvider
		bufregistryapimodule.V1Beta1UploadServiceClientProvider
	}
	publicRegistry string
}

func newUploader(
	logger *slog.Logger,
	moduleClientProvider interface {
		bufregistryapimodule.V1ModuleServiceClientProvider
		bufregistryapimodule.V1UploadServiceClientProvider
		bufregistryapimodule.V1Beta1UploadServiceClientProvider
	},
	options ...UploaderOption,
) *uploader {
	uploader := &uploader{
		logger:               logger,
		moduleClientProvider: moduleClientProvider,
		publicRegistry:       defaultPublicRegistry,
	}
	for _, option := range options {
		option(uploader)
	}
	return uploader
}

func (a *uploader) Upload(
	ctx context.Context,
	moduleSet bufmodule.ModuleSet,
	options ...bufmodule.UploadOption,
) ([]bufmodule.Commit, error) {
	uploadOptions, err := bufmodule.NewUploadOptions(options)
	if err != nil {
		return nil, err
	}

	contentModules, err := bufmodule.ModuleSetTargetLocalModulesAndTransitiveLocalDeps(moduleSet)
	if err != nil {
		return nil, err
	}
	// Only push named modules to the registry. Any dependencies for named modules must have a name.
	// Local unnamed modules can be excluded if the UploadWithExcludeUnnamed option is set.
	contentModules, err = xslices.FilterError(contentModules, func(module bufmodule.Module) (bool, error) {
		moduleName := module.FullName()
		if moduleName == nil {
			moduleDescription := module.Description()
			if uploadOptions.ExcludeUnnamed() {
				a.logger.Warn("Excluding unnamed module", slog.String("module", moduleDescription))
				return false, nil
			}
			return false, fmt.Errorf("a name must be specified in buf.yaml to push module: %s", moduleDescription)
		}
		deps, err := module.ModuleDeps()
		if err != nil {
			return false, err
		}
		if allDepModuleDescriptions := xslices.Reduce(deps, func(allDepModuleDescriptions []string, dep bufmodule.ModuleDep) []string {
			if moduleName := dep.FullName(); moduleName == nil {
				return append(allDepModuleDescriptions, dep.Description())
			}
			return allDepModuleDescriptions
		}, nil); len(allDepModuleDescriptions) > 0 {
			return false, fmt.Errorf(
				"all dependencies for module %q must be named but these modules are not:\n%s",
				moduleName.String(),
				strings.Join(
					xslices.Map(allDepModuleDescriptions, func(moduleDescription string) string { return "  " + moduleDescription }),
					"\n",
				),
			)
		}
		return true, nil
	})
	if err != nil {
		return nil, err
	}
	if len(contentModules) == 0 {
		// Nothing to upload.
		return nil, nil
	}
	primaryRegistry, err := getSingleRegistryForContentModules(contentModules)
	if err != nil {
		return nil, err
	}

	// This must be in the same order as contentModules.
	var modules []*modulev1.Module
	if uploadOptions.CreateIfNotExist() {
		// We must attempt to create each module one at a time, since CreateModules will return
		// an `AlreadyExists` if any of the modules we are attempting to create already exists,
		// and no new modules will be created.
		modules = make([]*modulev1.Module, len(contentModules))
		for i, contentModule := range contentModules {
			module, err := a.createContentModuleIfNotExist(
				ctx,
				primaryRegistry,
				contentModule,
				uploadOptions.CreateModuleVisibility(),
				uploadOptions.CreateDefaultLabel(),
			)
			if err != nil {
				return nil, err
			}
			modules[i] = module
		}
	} else {
		// The modules retrieved by GetModules retains the same order as the request, so
		// this matches the order of contentModules.
		modules, err = a.validateContentModulesExist(
			ctx,
			primaryRegistry,
			contentModules,
		)
		if err != nil {
			return nil, err
		}
	}

	var v1beta1ProtoScopedLabelRefs []*modulev1beta1.ScopedLabelRef
	if len(uploadOptions.Tags()) > 0 {
		contentModuleSortedDefaultLabels := xslices.ToUniqueSorted(
			xslices.Map(
				modules,
				func(module *modulev1.Module) string {
					return module.DefaultLabelName
				},
			),
		)
		if len(contentModuleSortedDefaultLabels) > 1 {
			return nil, fmt.Errorf(
				`--tag was used, but modules %q had multiple default tags %q. If multiple modules are being pushed and --tag is used, all modules must have the same default label.`,
				strings.Join(xslices.Map(
					contentModules,
					func(module bufmodule.Module) string {
						return module.FullName().String()
					},
				), ", "),
				strings.Join(contentModuleSortedDefaultLabels, ", "),
			)
		}
		if len(contentModuleSortedDefaultLabels) < 1 {
			// This should never happen, every module should have a default label, so we return
			// a syserror.
			return nil, syserror.New("no default labels for modules")
		}
		// While the API allows different labels per reference, we don't expose this through
		// the use of the `--label` flag, so all references will have the same labels.
		// We just pre-compute them now.
		labelNames := append(uploadOptions.Tags(), contentModuleSortedDefaultLabels[0])
		v1beta1ProtoScopedLabelRefs = xslices.Map(
			labelNames,
			labelNameToV1Beta1ProtoScopedLabelRef,
		)
	} else {
		// While the API allows different labels per reference, we don't expose this through
		// the use of the `--label` flag, so all references will have the same labels.
		// We just pre-compute them now.
		if len(uploadOptions.Labels()) > 0 {
			v1beta1ProtoScopedLabelRefs = xslices.Map(
				uploadOptions.Labels(),
				labelNameToV1Beta1ProtoScopedLabelRef,
			)
		}
	}
	// Maintains ordering, important for when we create bufmodule.Commit objects below.
	v1beta1ProtoUploadRequestContents, err := xslices.MapError(
		contentModules,
		func(module bufmodule.Module) (*modulev1beta1.UploadRequest_Content, error) {
			return getV1Beta1ProtoUploadRequestContent(
				ctx,
				v1beta1ProtoScopedLabelRefs,
				primaryRegistry,
				module,
				uploadOptions.SourceControlURL(),
			)
		},
	)
	if err != nil {
		return nil, err
	}

	remoteDeps, err := bufmodule.RemoteDepsForModules(contentModules)
	if err != nil {
		return nil, err
	}

	v1beta1ProtoUploadRequestDepRefs, err := xslices.MapError(
		remoteDeps,
		remoteDepToV1Beta1ProtoUploadRequestDepRef,
	)
	if err != nil {
		return nil, err
	}

	// A sorted slice of unique registries for the RemoteDeps.
	remoteDepRegistries := xslices.MapKeysToSortedSlice(
		// A map from registry to RemoteDeps for that registry.
		xslices.ToValuesMap(
			remoteDeps,
			func(remoteDep bufmodule.RemoteDep) string {
				// We've already validated two or three times that FullName is present here.
				return remoteDep.FullName().Registry()
			},
		),
	)
	if err := validateDepRegistries(primaryRegistry, remoteDepRegistries, a.publicRegistry); err != nil {
		return nil, err
	}

	var universalProtoCommits []*universalProtoCommit
	if len(remoteDepRegistries) > 0 && (len(remoteDepRegistries) > 1 || remoteDepRegistries[0] != primaryRegistry) {
		// If we have dependencies on other registries, or we have multiple registries we depend on, we have
		// to use legacy federation.
		response, err := a.moduleClientProvider.V1Beta1UploadServiceClient(primaryRegistry).Upload(
			ctx,
			connect.NewRequest(
				&modulev1beta1.UploadRequest{
					Contents: v1beta1ProtoUploadRequestContents,
					DepRefs:  v1beta1ProtoUploadRequestDepRefs,
				},
			),
		)
		if err != nil {
			return nil, err
		}
		universalProtoCommits, err = xslices.MapError(response.Msg.Commits, newUniversalProtoCommitForV1Beta1)
		if err != nil {
			return nil, err
		}
	} else {
		// If we only have a single registry, invoke the new API endpoint that does not allow
		// for federation. Do this so that we can maintain federated API endpoint metrics.
		//
		// Maintains ordering, important for when we create bufmodule.Commit objects below.
		v1ProtoUploadRequestContents := xslices.Map(
			v1beta1ProtoUploadRequestContents,
			v1beta1ProtoUploadRequestContentToV1ProtoUploadRequestContent,
		)
		protoDepCommitIds := xslices.Map(
			v1beta1ProtoUploadRequestDepRefs,
			func(v1beta1ProtoDepRef *modulev1beta1.UploadRequest_DepRef) string {
				return v1beta1ProtoDepRef.CommitId
			},
		)
		response, err := a.moduleClientProvider.V1UploadServiceClient(primaryRegistry).Upload(
			ctx,
			connect.NewRequest(
				&modulev1.UploadRequest{
					Contents:     v1ProtoUploadRequestContents,
					DepCommitIds: protoDepCommitIds,
				},
			),
		)
		if err != nil {
			return nil, err
		}
		universalProtoCommits, err = xslices.MapError(response.Msg.Commits, newUniversalProtoCommitForV1)
		if err != nil {
			return nil, err
		}
	}

	if len(universalProtoCommits) != len(v1beta1ProtoUploadRequestContents) {
		return nil, fmt.Errorf("expected %d Commits, got %d", len(v1beta1ProtoUploadRequestContents), len(universalProtoCommits))
	}
	commits := make([]bufmodule.Commit, len(universalProtoCommits))
	for i, universalProtoCommit := range universalProtoCommits {
		// This is how we get the FullName without calling the ModuleService or OwnerService.
		//
		// We've maintained ordering throughout this function, so we can do this.
		// The API returns Commits in the same order as the Contents.
		moduleFullName := contentModules[i].FullName()
		commitID, err := uuidutil.FromDashless(universalProtoCommit.ID)
		if err != nil {
			return nil, err
		}
		moduleKey, err := bufmodule.NewModuleKey(
			moduleFullName,
			commitID,
			func() (bufmodule.Digest, error) {
				return universalProtoCommit.Digest, nil
			},
		)
		if err != nil {
			return nil, err
		}
		commits[i] = bufmodule.NewCommit(
			moduleKey,
			func() (time.Time, error) {
				return universalProtoCommit.CreateTime, nil
			},
		)
	}
	return commits, nil
}

func (a *uploader) createContentModuleIfNotExist(
	ctx context.Context,
	primaryRegistry string,
	contentModule bufmodule.Module,
	createModuleVisibility bufmodule.ModuleVisibility,
	createDefaultLabel string,
) (*modulev1.Module, error) {
	v1ProtoCreateModuleVisibility, err := moduleVisibilityToV1Proto(createModuleVisibility)
	if err != nil {
		return nil, err
	}
	response, err := a.moduleClientProvider.V1ModuleServiceClient(primaryRegistry).CreateModules(
		ctx,
		connect.NewRequest(
			&modulev1.CreateModulesRequest{
				Values: []*modulev1.CreateModulesRequest_Value{
					{
						OwnerRef: &ownerv1.OwnerRef{
							Value: &ownerv1.OwnerRef_Name{
								Name: contentModule.FullName().Owner(),
							},
						},
						Name:             contentModule.FullName().Name(),
						Visibility:       v1ProtoCreateModuleVisibility,
						DefaultLabelName: createDefaultLabel,
					},
				},
			},
		),
	)
	if err != nil {
		if connect.CodeOf(err) == connect.CodeAlreadyExists {
			// If a module already existed, then we check validate its contents.
			modules, err := a.validateContentModulesExist(ctx, primaryRegistry, []bufmodule.Module{contentModule})
			if err != nil {
				return nil, err
			}
			if len(modules) != 1 {
				return nil, syserror.Newf("expected 1 Module, found %d", len(modules))
			}
			return modules[0], nil
		}
		return nil, err
	}
	if len(response.Msg.Modules) != 1 {
		return nil, syserror.Newf("expected 1 Module, found %d", len(response.Msg.Modules))
	}
	// Otherwise we return the module we created
	return response.Msg.Modules[0], nil
}

func (a *uploader) validateContentModulesExist(
	ctx context.Context,
	primaryRegistry string,
	contentModules []bufmodule.Module,
) ([]*modulev1.Module, error) {
	response, err := a.moduleClientProvider.V1ModuleServiceClient(primaryRegistry).GetModules(
		ctx,
		connect.NewRequest(
			&modulev1.GetModulesRequest{
				ModuleRefs: xslices.Map(
					contentModules,
					func(module bufmodule.Module) *modulev1.ModuleRef {
						return &modulev1.ModuleRef{
							Value: &modulev1.ModuleRef_Name_{
								Name: &modulev1.ModuleRef_Name{
									Owner:  module.FullName().Owner(),
									Module: module.FullName().Name(),
								},
							},
						}
					},
				),
			},
		),
	)
	if err != nil {
		return nil, err
	}
	return response.Msg.Modules, nil
}

func getV1Beta1ProtoUploadRequestContent(
	ctx context.Context,
	v1beta1ProtoScopedLabelRefs []*modulev1beta1.ScopedLabelRef,
	primaryRegistry string,
	module bufmodule.Module,
	sourceControlURL string,
) (*modulev1beta1.UploadRequest_Content, error) {
	if !module.IsLocal() {
		return nil, syserror.New("expected local Module in getProtoLegacyFederationUploadRequestContent")
	}
	if module.FullName() == nil {
		return nil, syserror.Newf("expected module name for local module: %s", module.Description())
	}
	if module.FullName().Registry() != primaryRegistry {
		// This should never happen - the upload Modules should already be verified above to come from one registry.
		return nil, syserror.Newf("attempting to upload content for registry other than %s in getProtoLegacyFederationUploadRequestContent", primaryRegistry)
	}

	v1beta1ProtoFiles, err := bucketToV1Beta1ProtoFiles(ctx, bufmodule.ModuleReadBucketToStorageReadBucket(module))
	if err != nil {
		return nil, err
	}

	uploadRequestContent := &modulev1beta1.UploadRequest_Content{
		ModuleRef: &modulev1beta1.ModuleRef{
			Value: &modulev1beta1.ModuleRef_Name_{
				Name: &modulev1beta1.ModuleRef_Name{
					Owner:  module.FullName().Owner(),
					Module: module.FullName().Name(),
				},
			},
		},
		Files:           v1beta1ProtoFiles,
		ScopedLabelRefs: v1beta1ProtoScopedLabelRefs,
	}
	if sourceControlURL != "" {
		uploadRequestContent.SourceControlUrl = sourceControlURL
	}
	return uploadRequestContent, nil
}

func remoteDepToV1Beta1ProtoUploadRequestDepRef(
	remoteDep bufmodule.RemoteDep,
) (*modulev1beta1.UploadRequest_DepRef, error) {
	if remoteDep.FullName() == nil {
		return nil, syserror.Newf("expected module name for remote module dependency %q", remoteDep.OpaqueID())
	}
	depCommitID := remoteDep.CommitID()
	if depCommitID == uuid.Nil {
		return nil, syserror.Newf("did not have a commit ID for a remote module dependency %q", remoteDep.OpaqueID())
	}
	return &modulev1beta1.UploadRequest_DepRef{
		CommitId: uuidutil.ToDashless(depCommitID),
		Registry: remoteDep.FullName().Registry(),
	}, nil
}
