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

package moduleundeprecate

import (
	"context"
	"fmt"

	modulev1 "buf.build/gen/go/bufbuild/registry/protocolbuffers/go/buf/registry/module/v1"
	"buf.build/go/app/appcmd"
	"buf.build/go/app/appext"
	"connectrpc.com/connect"
	"github.com/bufbuild/buf/private/buf/bufcli"
	"github.com/bufbuild/buf/private/bufpkg/bufparse"
	"github.com/bufbuild/buf/private/bufpkg/bufregistryapi/bufregistryapimodule"
	"github.com/bufbuild/buf/private/pkg/syserror"
)

// NewCommand returns a new Command
func NewCommand(name string, builder appext.SubCommandBuilder) *appcmd.Command {
	return &appcmd.Command{
		Use:   name + " <buf.build/owner/module>",
		Short: "Undeprecate a BSR module",
		Args:  appcmd.ExactArgs(1),
		Run:   builder.NewRunFunc(run),
	}
}

func run(ctx context.Context, container appext.Container) error {
	moduleFullName, err := bufparse.ParseFullName(container.Arg(0))
	if err != nil {
		return appcmd.WrapInvalidArgumentError(err)
	}
	clientConfig, err := bufcli.NewConnectClientConfig(container)
	if err != nil {
		return err
	}
	moduleClientProvider := bufregistryapimodule.NewClientProvider(clientConfig)
	moduleServiceClient := moduleClientProvider.V1ModuleServiceClient(moduleFullName.Registry())
	if _, err := moduleServiceClient.UpdateModules(
		ctx,
		&connect.Request[modulev1.UpdateModulesRequest]{
			Msg: &modulev1.UpdateModulesRequest{
				Values: []*modulev1.UpdateModulesRequest_Value{
					{
						ModuleRef: &modulev1.ModuleRef{
							Value: &modulev1.ModuleRef_Name_{
								Name: &modulev1.ModuleRef_Name{
									Owner:  moduleFullName.Owner(),
									Module: moduleFullName.Name(),
								},
							},
						},
						State: modulev1.ModuleState_MODULE_STATE_ACTIVE.Enum(),
					},
				},
			},
		},
	); err != nil {
		if connect.CodeOf(err) == connect.CodeNotFound {
			return bufcli.NewModuleNotFoundError(container.Arg(0))
		}
		return err
	}
	if _, err := fmt.Fprintln(container.Stdout(), "Module undeprecated."); err != nil {
		return syserror.Wrap(err)
	}
	return nil
}
