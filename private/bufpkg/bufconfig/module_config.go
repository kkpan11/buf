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

package bufconfig

import (
	"errors"
	"fmt"

	"github.com/bufbuild/buf/private/bufpkg/bufparse"
	"github.com/bufbuild/buf/private/pkg/normalpath"
	"github.com/bufbuild/buf/private/pkg/standard/xslices"
)

var (
	// DefaultModuleConfigV1 is the default ModuleConfig for v1.
	DefaultModuleConfigV1 ModuleConfig

	// DefaultModuleConfigV2 is the default ModuleConfig for v1.
	DefaultModuleConfigV2 ModuleConfig
)

func init() {
	var err error
	DefaultModuleConfigV1, err = newModuleConfig(
		".",
		nil,
		map[string][]string{
			".": {},
		},
		map[string][]string{
			".": {},
		},
		DefaultLintConfigV1,
		DefaultBreakingConfigV1,
	)
	if err != nil {
		panic(err.Error())
	}
	DefaultModuleConfigV2, err = newModuleConfig(
		".",
		nil,
		map[string][]string{
			".": {},
		},
		map[string][]string{
			".": {},
		},
		DefaultLintConfigV2,
		DefaultBreakingConfigV2,
	)
	if err != nil {
		panic(err.Error())
	}
}

// ModuleConfig is configuration for a specific Module.
//
// ModuleConfigs do not expose BucketID or OpaqueID, however DirPath is effectively BucketID,
// and FullName -> fallback to DirPath effectively is OpaqueID. Given that it is up to
// the user of this package to decide what to do with these fields, we do not name DirPath as
// BucketID, and we do not expose OpaqueID.
type ModuleConfig interface {
	// DirPath returns the path of the Module within the Workspace, if specified.
	//
	// This is always present. For v1beta1 and v1 buf.yamls, this is always ".".
	//
	// In v2, this will be used as the BucketID within Workspaces. For v1, it is up
	// to the Workspace constructor to come up with a BucketID (likely the directory name
	// within buf.work.yaml).
	DirPath() string
	// FullName returns the FullName for the Module, if available.
	//
	// This may be nil.
	FullName() bufparse.FullName
	// RootToIncludes contains a map from root to the directories to include for that root.
	// The keys in RootToIncludes are always the same as those in RooToExcludes.
	//
	// Roots are the root directories within a bucket to search for Protobuf files.
	//
	// There will be no between the roots, ie foo/bar and foo are not allowed.
	// All Protobuf files must be unique relative to the roots, ie if foo and bar
	// are roots, then foo/baz.proto and bar/baz.proto are not allowed.
	// All roots will be normalized and validated.
	//
	// A proto file within a root is considered part of the module when it satisfies both:
	//
	// - being inside ONE of the include paths for this root (unless includes is empty, which
	//   does not filter the proto files)
	// - being inside NONE of the exclude paths for this root
	//
	// There should be no overlap between the includes, ie foo/bar and foo are not allowed.
	// All includes must reside within a root, but none will be equal to a root.
	// All includes will be normalized and validated.
	//
	// *** The includes in this map will be relative to the root they map to! ***
	// *** Note that root is relative to DirPath! ***
	// That is, the actual path to a root within a is DirPath()/root, and the
	// actual path to an exclude is DirPath()/root/include (in v1beta1 and v1, this
	// is just root and root/exclude).
	//
	// This will never return a nil or empty value.
	// If RootToIncludes is empty in the buf.yaml, this will return "." -> []string{}.
	//
	// For v1beta1, this may contain multiple keys but the values for these keys are empty slices.
	// For v1, this will contain a single key "." with an empty slice as its value.
	// For v2, this will contain a single key ".", with potentially some includes.
	RootToIncludes() map[string][]string
	// RootToExcludes contains a map from root to the excludes for that root.
	// The keys in RootToExcludes are always the same as those in RootToIncludes.
	//
	// Excludes are the directories within a bucket to exclude.
	// There should be no overlap between the excludes, ie foo/bar and foo are not allowed.
	// All excludes must reside within a root, but none will be equal to a root.
	// All excludes will be normalized and validated.
	//
	// *** The excludes in this map will be relative to the root they map to! ***
	// *** Note that root is relative to DirPath! ***
	// That is, the actual path to a root within a is DirPath()/root, and the
	// actual path to an exclude is DirPath()/root/exclude (in v1beta1 and v1, this
	// is just root and root/exclude).
	//
	// This will never return a nil or empty value.
	// If RootToExcludes is empty in the buf.yaml, this will return "." -> []string{}.
	//
	// For v1beta1, this may contain multiple keys.
	// For v1 and v2, this will contain a single key ".", with potentially some excludes.
	RootToExcludes() map[string][]string
	// LintConfig returns the lint configuration.
	//
	// If this was not set, this will be set to the default lint configuration.
	LintConfig() LintConfig
	// BreakingConfig returns the breaking configuration.
	//
	// If this was not set, this will be set to the default breaking configuration.
	BreakingConfig() BreakingConfig

	isModuleConfig()
}

// NewModuleConfig returns a new ModuleConfig.
func NewModuleConfig(
	dirPath string,
	moduleFullName bufparse.FullName,
	rootToIncludes map[string][]string,
	rootToExcludes map[string][]string,
	lintConfig LintConfig,
	breakingConfig BreakingConfig,
) (ModuleConfig, error) {
	return newModuleConfig(
		dirPath,
		moduleFullName,
		rootToIncludes,
		rootToExcludes,
		lintConfig,
		breakingConfig,
	)
}

// *** PRIVATE ***

type moduleConfig struct {
	dirPath        string
	moduleFullName bufparse.FullName
	rootToIncludes map[string][]string
	rootToExcludes map[string][]string
	lintConfig     LintConfig
	breakingConfig BreakingConfig
}

// All validations are syserrors as we only ever read ModuleConfigs.
func newModuleConfig(
	dirPath string,
	moduleFullName bufparse.FullName,
	rootToIncludes map[string][]string,
	rootToExcludes map[string][]string,
	lintConfig LintConfig,
	breakingConfig BreakingConfig,
) (*moduleConfig, error) {
	// Returns "." on empty input.
	dirPath, err := normalpath.NormalizeAndValidate(dirPath)
	if err != nil {
		return nil, err
	}
	if lintConfig == nil {
		return nil, errors.New("LintConfig was nil")
	}
	if breakingConfig == nil {
		return nil, errors.New("BreakingConfig was nil")
	}
	lintFileVersion := lintConfig.FileVersion()
	breakingFileVersion := breakingConfig.FileVersion()
	if lintFileVersion != breakingFileVersion {
		return nil, fmt.Errorf(
			"LintConfig FileVersion %v did not match BreakingConfig FileVersion %v",
			lintFileVersion,
			breakingFileVersion,
		)
	}
	fileVersion := lintFileVersion
	if fileVersion == FileVersionV1Beta1 || fileVersion == FileVersionV1 {
		if dirPath != "." {
			return nil, fmt.Errorf("had dirPath %q for NewModuleConfig with FileVersion %v", dirPath, fileVersion)
		}
	}
	if fileVersion == FileVersionV1 || fileVersion == FileVersionV2 {
		if len(rootToExcludes) != 1 {
			return nil, fmt.Errorf("had rootToExcludes length %d for NewModuleConfig with FileVersion %v", len(rootToExcludes), fileVersion)
		}
		if _, ok := rootToExcludes["."]; !ok {
			return nil, fmt.Errorf("had rootToExcludes without key \".\" for NewModuleConfig with FileVersion %v", fileVersion)
		}
	}
	newRootToIncludes := make(map[string][]string)
	for root, includes := range rootToIncludes {
		includes, err := xslices.MapError(includes, normalpath.NormalizeAndValidate)
		if err != nil {
			return nil, err
		}
		newRootToIncludes[root] = xslices.ToUniqueSorted(includes)
	}
	newRootToExcludes := make(map[string][]string)
	for root, excludes := range rootToExcludes {
		excludes, err := xslices.MapError(excludes, normalpath.NormalizeAndValidate)
		if err != nil {
			return nil, err
		}
		newRootToExcludes[root] = xslices.ToUniqueSorted(excludes)
	}
	return &moduleConfig{
		dirPath:        dirPath,
		moduleFullName: moduleFullName,
		rootToIncludes: newRootToIncludes,
		rootToExcludes: newRootToExcludes,
		lintConfig:     lintConfig,
		breakingConfig: breakingConfig,
	}, nil
}

func (m *moduleConfig) DirPath() string {
	return m.dirPath
}

func (m *moduleConfig) FullName() bufparse.FullName {
	return m.moduleFullName
}

func (m *moduleConfig) RootToIncludes() map[string][]string {
	return copyStringToStringSliceMap(m.rootToIncludes)
}

func (m *moduleConfig) RootToExcludes() map[string][]string {
	return copyStringToStringSliceMap(m.rootToExcludes)
}

func (m *moduleConfig) LintConfig() LintConfig {
	return m.lintConfig
}

func (m *moduleConfig) BreakingConfig() BreakingConfig {
	return m.breakingConfig
}

func (*moduleConfig) isModuleConfig() {}
