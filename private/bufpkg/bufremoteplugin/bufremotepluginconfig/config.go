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

package bufremotepluginconfig

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"buf.build/go/spdx"
	"github.com/bufbuild/buf/private/bufpkg/bufremoteplugin/bufremotepluginref"
	"github.com/bufbuild/buf/private/pkg/standard/xslices"
	"golang.org/x/mod/modfile"
	"golang.org/x/mod/semver"
)

func newConfig(externalConfig ExternalConfig, options []ConfigOption) (*Config, error) {
	opts := &configOptions{}
	for _, option := range options {
		option(opts)
	}
	pluginIdentity, err := pluginIdentityForStringWithOverrideRemote(externalConfig.Name, opts.overrideRemote)
	if err != nil {
		return nil, err
	}
	pluginVersion := externalConfig.PluginVersion
	if pluginVersion == "" {
		return nil, errors.New("a plugin_version is required")
	}
	if !semver.IsValid(pluginVersion) {
		return nil, fmt.Errorf("plugin_version %q must be a valid semantic version", externalConfig.PluginVersion)
	}
	var dependencies []bufremotepluginref.PluginReference
	if len(externalConfig.Deps) > 0 {
		existingDeps := make(map[string]struct{})
		for _, dependency := range externalConfig.Deps {
			reference, err := pluginReferenceForStringWithOverrideRemote(dependency.Plugin, dependency.Revision, opts.overrideRemote)
			if err != nil {
				return nil, err
			}
			if reference.Remote() != pluginIdentity.Remote() {
				return nil, fmt.Errorf("plugin dependency %q must use same remote as plugin %q", dependency, pluginIdentity.Remote())
			}
			if _, ok := existingDeps[reference.IdentityString()]; ok {
				return nil, fmt.Errorf("plugin dependency %q was specified more than once", dependency)
			}
			existingDeps[reference.IdentityString()] = struct{}{}
			dependencies = append(dependencies, reference)
		}
	}
	registryConfig, err := newRegistryConfig(externalConfig.Registry, dependencies, opts.overrideRemote)
	if err != nil {
		return nil, err
	}
	spdxLicenseID := externalConfig.SPDXLicenseID
	if spdxLicenseID != "" {
		if license, ok := spdx.LicenseForID(spdxLicenseID); ok {
			spdxLicenseID = license.ID
		} else {
			return nil, fmt.Errorf("unknown SPDX License ID %q", spdxLicenseID)
		}
	}
	return &Config{
		Name:                pluginIdentity,
		PluginVersion:       pluginVersion,
		Dependencies:        dependencies,
		Registry:            registryConfig,
		SourceURL:           externalConfig.SourceURL,
		Description:         externalConfig.Description,
		OutputLanguages:     externalConfig.OutputLanguages,
		SPDXLicenseID:       spdxLicenseID,
		LicenseURL:          externalConfig.LicenseURL,
		IntegrationGuideURL: externalConfig.IntegrationGuideURL,
		Deprecated:          externalConfig.Deprecated,
	}, nil
}

func newRegistryConfig(
	externalRegistryConfig ExternalRegistryConfig,
	pluginDependencies []bufremotepluginref.PluginReference,
	overrideRemote string,
) (*RegistryConfig, error) {
	var (
		isGoEmpty     = externalRegistryConfig.Go == nil
		isNPMEmpty    = externalRegistryConfig.NPM == nil
		isMavenEmpty  = externalRegistryConfig.Maven == nil
		isSwiftEmpty  = externalRegistryConfig.Swift == nil
		isPythonEmpty = externalRegistryConfig.Python == nil
		isCargoEmpty  = externalRegistryConfig.Cargo == nil
		isNugetEmpty  = externalRegistryConfig.Nuget == nil
		isCmakeEmpty  = externalRegistryConfig.Cmake == nil
	)
	var registryCount int
	for _, isEmpty := range []bool{
		isGoEmpty,
		isNPMEmpty,
		isMavenEmpty,
		isSwiftEmpty,
		isPythonEmpty,
		isCargoEmpty,
		isNugetEmpty,
		isCmakeEmpty,
	} {
		if !isEmpty {
			registryCount++
		}
		if registryCount > 1 {
			// We might eventually want to support multiple runtime configuration,
			// but it's safe to start with an error for now.
			return nil, fmt.Errorf("%s configuration contains multiple registry configurations", ExternalConfigFilePath)
		}
	}
	if registryCount == 0 {
		// It's possible that the plugin doesn't have any runtime dependencies.
		return nil, nil
	}
	options := OptionsSliceToPluginOptions(externalRegistryConfig.Opts)
	switch {
	case !isGoEmpty:
		goRegistryConfig, err := newGoRegistryConfig(externalRegistryConfig.Go, pluginDependencies, overrideRemote)
		if err != nil {
			return nil, err
		}
		return &RegistryConfig{
			Go:      goRegistryConfig,
			Options: options,
		}, nil
	case !isNPMEmpty:
		npmRegistryConfig, err := newNPMRegistryConfig(externalRegistryConfig.NPM)
		if err != nil {
			return nil, err
		}
		return &RegistryConfig{
			NPM:     npmRegistryConfig,
			Options: options,
		}, nil
	case !isMavenEmpty:
		mavenRegistryConfig, err := newMavenRegistryConfig(externalRegistryConfig.Maven)
		if err != nil {
			return nil, err
		}
		return &RegistryConfig{
			Maven:   mavenRegistryConfig,
			Options: options,
		}, nil
	case !isSwiftEmpty:
		swiftRegistryConfig, err := newSwiftRegistryConfig(externalRegistryConfig.Swift)
		if err != nil {
			return nil, err
		}
		return &RegistryConfig{
			Swift:   swiftRegistryConfig,
			Options: options,
		}, nil
	case !isPythonEmpty:
		pythonRegistryConfig, err := newPythonRegistryConfig(externalRegistryConfig.Python)
		if err != nil {
			return nil, err
		}
		return &RegistryConfig{
			Python:  pythonRegistryConfig,
			Options: options,
		}, nil
	case !isCargoEmpty:
		cargoRegistryConfig, err := newCargoRegistryConfig(externalRegistryConfig.Cargo)
		if err != nil {
			return nil, err
		}
		return &RegistryConfig{
			Cargo:   cargoRegistryConfig,
			Options: options,
		}, nil
	case !isNugetEmpty:
		nugetRegistryConfig, err := newNugetRegistryConfig(externalRegistryConfig.Nuget)
		if err != nil {
			return nil, err
		}
		return &RegistryConfig{
			Nuget:   nugetRegistryConfig,
			Options: options,
		}, nil
	case !isCmakeEmpty:
		cmakeRegistryConfig, err := newCmakeRegistryConfig(externalRegistryConfig.Cmake)
		if err != nil {
			return nil, err
		}
		return &RegistryConfig{
			Cmake:   cmakeRegistryConfig,
			Options: options,
		}, nil
	default:
		return nil, errors.New("unknown registry configuration")
	}
}

func newNPMRegistryConfig(externalNPMRegistryConfig *ExternalNPMRegistryConfig) (*NPMRegistryConfig, error) {
	if externalNPMRegistryConfig == nil {
		return nil, nil
	}
	var dependencies []*NPMRegistryDependencyConfig
	for _, dep := range externalNPMRegistryConfig.Deps {
		if dep.Package == "" {
			return nil, errors.New("npm runtime dependency requires a non-empty package name")
		}
		if dep.Version == "" {
			return nil, errors.New("npm runtime dependency requires a non-empty version name")
		}
		// TODO: Note that we don't have NPM-specific validation yet - any
		// non-empty string will work for the package and version.
		//
		// For a complete set of the version syntax we need to support, see
		// https://docs.npmjs.com/cli/v6/using-npm/semver
		//
		// https://github.com/Masterminds/semver might be a good candidate for
		// this, but it might not support all of the constraints supported
		// by NPM.
		dependencies = append(
			dependencies,
			&NPMRegistryDependencyConfig{
				Package: dep.Package,
				Version: dep.Version,
			},
		)
	}
	switch externalNPMRegistryConfig.ImportStyle {
	case "module", "commonjs":
	default:
		return nil, errors.New(`npm registry config import_style must be one of: "module" or "commonjs"`)
	}
	return &NPMRegistryConfig{
		RewriteImportPathSuffix: externalNPMRegistryConfig.RewriteImportPathSuffix,
		Deps:                    dependencies,
		ImportStyle:             externalNPMRegistryConfig.ImportStyle,
	}, nil
}

func newGoRegistryConfig(
	externalGoRegistryConfig *ExternalGoRegistryConfig,
	pluginDependencies []bufremotepluginref.PluginReference,
	overrideRemote string,
) (*GoRegistryConfig, error) {
	if externalGoRegistryConfig == nil {
		return nil, nil
	}
	if externalGoRegistryConfig.MinVersion != "" && !modfile.GoVersionRE.MatchString(externalGoRegistryConfig.MinVersion) {
		return nil, fmt.Errorf("the go minimum version %q must be a valid semantic version in the form of <major>.<minor>", externalGoRegistryConfig.MinVersion)
	}
	var runtimeDependencies []*GoRegistryDependencyConfig
	for _, dep := range externalGoRegistryConfig.Deps {
		if dep.Module == "" {
			return nil, errors.New("go runtime dependency requires a non-empty module name")
		}
		if dep.Version == "" {
			return nil, errors.New("go runtime dependency requires a non-empty version name")
		}
		if !semver.IsValid(dep.Version) {
			return nil, fmt.Errorf("go runtime dependency %s:%s does not have a valid semantic version", dep.Module, dep.Version)
		}
		runtimeDependencies = append(
			runtimeDependencies,
			&GoRegistryDependencyConfig{
				Module:  dep.Module,
				Version: dep.Version,
			},
		)
	}
	var basePlugin bufremotepluginref.PluginIdentity
	if externalGoRegistryConfig.BasePlugin != "" {
		var err error
		basePlugin, err = pluginIdentityForStringWithOverrideRemote(externalGoRegistryConfig.BasePlugin, overrideRemote)
		if err != nil {
			return nil, fmt.Errorf("failed to parse base plugin: %w", err)
		}
		// Validate the base plugin is included as one of the plugin dependencies when both are
		// specified. This ensures there's exactly one base type and it has a known dependency to
		// generate imports correctly and build a correct Go mod file.
		if len(pluginDependencies) > 0 {
			ok := slices.ContainsFunc(pluginDependencies, func(ref bufremotepluginref.PluginReference) bool {
				return ref.IdentityString() == basePlugin.IdentityString()
			})
			if !ok {
				return nil, fmt.Errorf("base plugin %q not found in plugin dependencies", externalGoRegistryConfig.BasePlugin)
			}
		}
	}
	return &GoRegistryConfig{
		MinVersion: externalGoRegistryConfig.MinVersion,
		Deps:       runtimeDependencies,
		BasePlugin: basePlugin,
	}, nil
}

func newMavenRegistryConfig(externalMavenRegistryConfig *ExternalMavenRegistryConfig) (*MavenRegistryConfig, error) {
	if externalMavenRegistryConfig == nil {
		return nil, nil
	}
	var dependencies []MavenDependencyConfig
	for _, externalDep := range externalMavenRegistryConfig.Deps {
		dep, err := mavenExternalDependencyToDependencyConfig(externalDep)
		if err != nil {
			return nil, err
		}
		dependencies = append(dependencies, dep)
	}
	var additionalRuntimes []MavenRuntimeConfig
	for _, runtime := range externalMavenRegistryConfig.AdditionalRuntimes {
		var deps []MavenDependencyConfig
		for _, externalDep := range runtime.Deps {
			dep, err := mavenExternalDependencyToDependencyConfig(externalDep)
			if err != nil {
				return nil, err
			}
			deps = append(deps, dep)
		}
		config := MavenRuntimeConfig{
			Name:    runtime.Name,
			Deps:    deps,
			Options: runtime.Opts,
		}
		additionalRuntimes = append(additionalRuntimes, config)
	}
	return &MavenRegistryConfig{
		Compiler: MavenCompilerConfig{
			Java: MavenCompilerJavaConfig{
				Encoding: externalMavenRegistryConfig.Compiler.Java.Encoding,
				Release:  externalMavenRegistryConfig.Compiler.Java.Release,
				Source:   externalMavenRegistryConfig.Compiler.Java.Source,
				Target:   externalMavenRegistryConfig.Compiler.Java.Target,
			},
			Kotlin: MavenCompilerKotlinConfig{
				APIVersion:      externalMavenRegistryConfig.Compiler.Kotlin.APIVersion,
				JVMTarget:       externalMavenRegistryConfig.Compiler.Kotlin.JVMTarget,
				LanguageVersion: externalMavenRegistryConfig.Compiler.Kotlin.LanguageVersion,
				Version:         externalMavenRegistryConfig.Compiler.Kotlin.Version,
			},
		},
		Deps:               dependencies,
		AdditionalRuntimes: additionalRuntimes,
	}, nil
}

func newSwiftRegistryConfig(externalSwiftRegistryConfig *ExternalSwiftRegistryConfig) (*SwiftRegistryConfig, error) {
	if externalSwiftRegistryConfig == nil {
		return nil, nil
	}
	var dependencies []SwiftRegistryDependencyConfig
	for _, externalDependency := range externalSwiftRegistryConfig.Deps {
		dependency, err := swiftExternalDependencyToDependencyConfig(externalDependency)
		if err != nil {
			return nil, err
		}
		dependencies = append(dependencies, dependency)
	}
	return &SwiftRegistryConfig{
		Dependencies: dependencies,
	}, nil
}

func swiftExternalDependencyToDependencyConfig(externalDep ExternalSwiftRegistryDependencyConfig) (SwiftRegistryDependencyConfig, error) {
	if externalDep.Source == "" {
		return SwiftRegistryDependencyConfig{}, errors.New("swift runtime dependency requires a non-empty source")
	}
	if externalDep.Package == "" {
		return SwiftRegistryDependencyConfig{}, errors.New("swift runtime dependency requires a non-empty package name")
	}
	if externalDep.Version == "" {
		return SwiftRegistryDependencyConfig{}, errors.New("swift runtime dependency requires a non-empty version name")
	}
	// Swift SemVers are typically not prefixed with a "v". The Golang semver library requires a "v" prefix.
	if !semver.IsValid(fmt.Sprintf("v%s", externalDep.Version)) {
		return SwiftRegistryDependencyConfig{}, fmt.Errorf("swift runtime dependency %s:%s does not have a valid semantic version", externalDep.Package, externalDep.Version)
	}
	return SwiftRegistryDependencyConfig{
		Source:        externalDep.Source,
		Package:       externalDep.Package,
		Version:       externalDep.Version,
		Products:      externalDep.Products,
		SwiftVersions: externalDep.SwiftVersions,
		Platforms: SwiftRegistryDependencyPlatformConfig{
			MacOS:   externalDep.Platforms.MacOS,
			IOS:     externalDep.Platforms.IOS,
			TVOS:    externalDep.Platforms.TVOS,
			WatchOS: externalDep.Platforms.WatchOS,
		},
	}, nil
}

func newPythonRegistryConfig(externalPythonRegistryConfig *ExternalPythonRegistryConfig) (*PythonRegistryConfig, error) {
	if externalPythonRegistryConfig == nil {
		return nil, nil
	}
	var dependencySpecifications []string
	for _, externalDependencySpecification := range externalPythonRegistryConfig.Deps {
		if externalDependencySpecification == "" {
			return nil, fmt.Errorf("python registry config cannot have an empty dependency specification")
		}
		dependencySpecifications = append(dependencySpecifications, externalDependencySpecification)
	}
	switch externalPythonRegistryConfig.PackageType {
	case "runtime", "stub-only":
	default:
		return nil, errors.New(`python registry config package_type must be one of: "runtime" or "stub-only"`)
	}
	return &PythonRegistryConfig{
		Deps:           dependencySpecifications,
		RequiresPython: externalPythonRegistryConfig.RequiresPython,
		PackageType:    externalPythonRegistryConfig.PackageType,
	}, nil
}

func newCargoRegistryConfig(externalCargoRegistryConfig *ExternalCargoRegistryConfig) (*CargoRegistryConfig, error) {
	if externalCargoRegistryConfig == nil {
		return nil, nil
	}
	config := &CargoRegistryConfig{
		RustVersion: externalCargoRegistryConfig.RustVersion,
	}
	for _, dependency := range externalCargoRegistryConfig.Deps {
		if dependency.Name == "" {
			return nil, fmt.Errorf("cargo registry dependency cannot have empty name")
		}
		if dependency.VersionRequirement == "" {
			return nil, fmt.Errorf("cargo registry dependency cannot have empty req")
		}
		config.Deps = append(config.Deps, CargoRegistryDependency(dependency))
	}
	return config, nil
}

func newNugetRegistryConfig(externalConfig *ExternalNugetRegistryConfig) (*NugetRegistryConfig, error) {
	if len(externalConfig.TargetFrameworks) == 0 {
		return nil, errors.New("nuget registry target frameworks required")
	}
	targetFrameworks, err := validateNugetTargetFrameworks(externalConfig.TargetFrameworks)
	if err != nil {
		return nil, fmt.Errorf("nuget registry: %w", err)
	}
	var deps []NugetDependencyConfig
	if len(externalConfig.Deps) > 0 {
		deps = make([]NugetDependencyConfig, 0, len(externalConfig.Deps))
		for i, externalDep := range externalConfig.Deps {
			if externalDep.Name == "" {
				return nil, fmt.Errorf("nuget registry dependency %d: empty name", i)
			}
			if externalDep.Version == "" {
				return nil, fmt.Errorf("nuget registry dependency %d: empty version", i)
			}
			depTargetFrameworks, err := validateNugetTargetFrameworks(externalDep.TargetFrameworks)
			if err != nil {
				return nil, fmt.Errorf("nuget registry dependency %d: %w", i, err)
			}
			deps = append(deps, NugetDependencyConfig{
				Name:             externalDep.Name,
				Version:          externalDep.Version,
				TargetFrameworks: depTargetFrameworks,
			})
		}
	}
	return &NugetRegistryConfig{
		TargetFrameworks: targetFrameworks,
		Deps:             deps,
	}, nil
}

func validateNugetTargetFrameworks(targetFrameworks []string) ([]string, error) {
	if len(targetFrameworks) == 0 {
		return nil, nil
	}
	if dups := xslices.Duplicates(targetFrameworks); len(dups) > 0 {
		return nil, fmt.Errorf("duplicate target frameworks: %v", dups)
	}
	for i, targetFramework := range targetFrameworks {
		if err := validateNugetTargetFramework(targetFramework); err != nil {
			return nil, fmt.Errorf("target framework %d: %w", i, err)
		}
	}
	return slices.Clone(targetFrameworks), nil
}

func validateNugetTargetFramework(targetFramework string) error {
	switch targetFramework {
	case "netstandard1.0":
	case "netstandard1.1":
	case "netstandard1.2":
	case "netstandard1.3":
	case "netstandard1.4":
	case "netstandard1.5":
	case "netstandard1.6":
	case "netstandard2.0":
	case "netstandard2.1":
	case "net5.0":
	case "net6.0":
	case "net7.0":
	case "net8.0":
	default:
		return fmt.Errorf("invalid target framework: %q", targetFramework)
	}
	return nil
}

func newCmakeRegistryConfig(externalCmakeRegistryConfig *ExternalCmakeRegistryConfig) (*CmakeRegistryConfig, error) {
	if externalCmakeRegistryConfig == nil {
		return nil, nil
	}
	return &CmakeRegistryConfig{}, nil
}

func pluginIdentityForStringWithOverrideRemote(identityStr string, overrideRemote string) (bufremotepluginref.PluginIdentity, error) {
	identity, err := bufremotepluginref.PluginIdentityForString(identityStr)
	if err != nil {
		return nil, err
	}
	if len(overrideRemote) == 0 {
		return identity, nil
	}
	return bufremotepluginref.NewPluginIdentity(overrideRemote, identity.Owner(), identity.Plugin())
}

func pluginReferenceForStringWithOverrideRemote(
	referenceStr string,
	revision int,
	overrideRemote string,
) (bufremotepluginref.PluginReference, error) {
	reference, err := bufremotepluginref.PluginReferenceForString(referenceStr, revision)
	if err != nil {
		return nil, err
	}
	if len(overrideRemote) == 0 {
		return reference, nil
	}
	overrideIdentity, err := pluginIdentityForStringWithOverrideRemote(reference.IdentityString(), overrideRemote)
	if err != nil {
		return nil, err
	}
	return bufremotepluginref.NewPluginReference(overrideIdentity, reference.Version(), reference.Revision())
}

func mavenExternalDependencyToDependencyConfig(dependency string) (MavenDependencyConfig, error) {
	// <groupId>:<artifactId>:<version>[:<classifier>][@<type>]
	dependencyWithoutExtension, extension, _ := strings.Cut(dependency, "@")
	components := strings.Split(dependencyWithoutExtension, ":")
	if len(components) < 3 {
		return MavenDependencyConfig{}, fmt.Errorf("invalid dependency %q: missing required groupId:artifactId:version fields", dependency)
	}
	if len(components) > 4 {
		return MavenDependencyConfig{}, fmt.Errorf("invalid dependency %q: maximum 4 fields before optional type", dependency)
	}
	config := MavenDependencyConfig{
		GroupID:    components[0],
		ArtifactID: components[1],
		Version:    components[2],
		Extension:  extension,
	}
	if len(components) == 4 {
		config.Classifier = components[3]
	}
	return config, nil
}
