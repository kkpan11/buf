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

package organizationcreate

import (
	"context"
	"fmt"

	ownerv1 "buf.build/gen/go/bufbuild/registry/protocolbuffers/go/buf/registry/owner/v1"
	"buf.build/go/app/appcmd"
	"buf.build/go/app/appext"
	"connectrpc.com/connect"
	"github.com/bufbuild/buf/private/buf/bufcli"
	"github.com/bufbuild/buf/private/buf/bufprint"
	"github.com/bufbuild/buf/private/bufpkg/bufregistryapi/bufregistryapiowner"
	"github.com/bufbuild/buf/private/pkg/syserror"
	"github.com/spf13/pflag"
)

const formatFlagName = "format"

// NewCommand returns a new Command
func NewCommand(
	name string,
	builder appext.SubCommandBuilder,
) *appcmd.Command {
	flags := newFlags()
	return &appcmd.Command{
		Use:   name + " <remote/organization>",
		Short: "Create a new BSR organization",
		Args:  appcmd.ExactArgs(1),
		Run: builder.NewRunFunc(
			func(ctx context.Context, container appext.Container) error {
				return run(ctx, container, flags)
			},
		),
		BindFlags: flags.Bind,
	}
}

type flags struct {
	Format string
}

func newFlags() *flags {
	return &flags{}
}

func (f *flags) Bind(flagSet *pflag.FlagSet) {
	flagSet.StringVar(
		&f.Format,
		formatFlagName,
		bufprint.FormatText.String(),
		fmt.Sprintf(`The output format to use. Must be one of %s`, bufprint.AllFormatsString),
	)
}

func run(
	ctx context.Context,
	container appext.Container,
	flags *flags,
) error {
	moduleOwner, err := bufcli.ParseModuleOwner(container.Arg(0))
	if err != nil {
		return appcmd.WrapInvalidArgumentError(err)
	}
	format, err := bufprint.ParseFormat(flags.Format)
	if err != nil {
		return appcmd.WrapInvalidArgumentError(err)
	}

	clientConfig, err := bufcli.NewConnectClientConfig(container)
	if err != nil {
		return err
	}
	ownerClientProvider := bufregistryapiowner.NewClientProvider(clientConfig)
	organizationServiceClient := ownerClientProvider.V1OrganizationServiceClient(moduleOwner.Registry())
	resp, err := organizationServiceClient.CreateOrganizations(
		ctx,
		connect.NewRequest(
			&ownerv1.CreateOrganizationsRequest{
				Values: []*ownerv1.CreateOrganizationsRequest_Value{
					{
						Name: moduleOwner.Owner(),
					},
				},
			},
		),
	)
	if err != nil {
		// Not explicitly handling error with connect.AlreadyExists as it can be a user or an
		// organization that already exists with that name.
		return err
	}
	organizations := resp.Msg.GetOrganizations()
	if len(organizations) != 1 {
		return syserror.Newf("unexpected number of organizations created by server: %d", len(organizations))
	}
	if format == bufprint.FormatText {
		if _, err := fmt.Fprintf(container.Stdout(), "Created %s.\n", moduleOwner); err != nil {
			return syserror.Wrap(err)
		}
		return nil
	}
	return bufprint.PrintNames(
		container.Stdout(),
		format,
		bufprint.NewOrganizationEntity(organizations[0], moduleOwner.Registry()),
	)
}
