// Copyright 2015 Google Inc. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package android

import (
	"fmt"
	"path"
	"path/filepath"
	"strings"
	"text/scanner"

	"github.com/google/blueprint"
	"github.com/google/blueprint/pathtools"
	"github.com/google/blueprint/proptools"
)

var (
	DeviceSharedLibrary = "shared_library"
	DeviceStaticLibrary = "static_library"
	DeviceExecutable    = "executable"
	HostSharedLibrary   = "host_shared_library"
	HostStaticLibrary   = "host_static_library"
	HostExecutable      = "host_executable"
)

type BuildParams struct {
	Rule            blueprint.Rule
	Deps            blueprint.Deps
	Depfile         WritablePath
	Description     string
	Output          WritablePath
	Outputs         WritablePaths
	ImplicitOutput  WritablePath
	ImplicitOutputs WritablePaths
	Input           Path
	Inputs          Paths
	Implicit        Path
	Implicits       Paths
	OrderOnly       Paths
	Default         bool
	Args            map[string]string
}

type ModuleBuildParams BuildParams

// BaseModuleContext is the same as blueprint.BaseModuleContext except that Config() returns
// a Config instead of an interface{}, and some methods have been wrapped to use an android.Module
// instead of a blueprint.Module, plus some extra methods that return Android-specific information
// about the current module.
type BaseModuleContext interface {
	Module() Module
	ModuleName() string
	ModuleDir() string
	ModuleType() string
	Config() Config

	OtherModuleName(m blueprint.Module) string
	OtherModuleDir(m blueprint.Module) string
	OtherModuleErrorf(m blueprint.Module, fmt string, args ...interface{})
	OtherModuleDependencyTag(m blueprint.Module) blueprint.DependencyTag
	OtherModuleExists(name string) bool
	OtherModuleType(m blueprint.Module) string

	GetDirectDepsWithTag(tag blueprint.DependencyTag) []Module
	GetDirectDepWithTag(name string, tag blueprint.DependencyTag) blueprint.Module
	GetDirectDep(name string) (blueprint.Module, blueprint.DependencyTag)

	VisitDirectDepsBlueprint(visit func(blueprint.Module))
	VisitDirectDeps(visit func(Module))
	VisitDirectDepsWithTag(tag blueprint.DependencyTag, visit func(Module))
	VisitDirectDepsIf(pred func(Module) bool, visit func(Module))
	// Deprecated: use WalkDeps instead to support multiple dependency tags on the same module
	VisitDepsDepthFirst(visit func(Module))
	// Deprecated: use WalkDeps instead to support multiple dependency tags on the same module
	VisitDepsDepthFirstIf(pred func(Module) bool, visit func(Module))
	WalkDeps(visit func(Module, Module) bool)
	WalkDepsBlueprint(visit func(blueprint.Module, blueprint.Module) bool)
	// GetWalkPath is supposed to be called in visit function passed in WalkDeps()
	// and returns a top-down dependency path from a start module to current child module.
	GetWalkPath() []Module

	ContainsProperty(name string) bool
	Errorf(pos scanner.Position, fmt string, args ...interface{})
	ModuleErrorf(fmt string, args ...interface{})
	PropertyErrorf(property, fmt string, args ...interface{})
	Failed() bool

	// GlobWithDeps returns a list of files that match the specified pattern but do not match any
	// of the patterns in excludes.  It also adds efficient dependencies to rerun the primary
	// builder whenever a file matching the pattern as added or removed, without rerunning if a
	// file that does not match the pattern is added to a searched directory.
	GlobWithDeps(pattern string, excludes []string) ([]string, error)

	Glob(globPattern string, excludes []string) Paths
	GlobFiles(globPattern string, excludes []string) Paths

	Fs() pathtools.FileSystem
	AddNinjaFileDeps(deps ...string)

	AddMissingDependencies(missingDeps []string)

	Target() Target
	TargetPrimary() bool
	MultiTargets() []Target
	Arch() Arch
	Os() OsType
	Host() bool
	Device() bool
	Darwin() bool
	Fuchsia() bool
	Windows() bool
	Debug() bool
	PrimaryArch() bool
	Platform() bool
	DeviceSpecific() bool
	SocSpecific() bool
	ProductSpecific() bool
	SystemExtSpecific() bool
	AConfig() Config
	DeviceConfig() DeviceConfig
}

// Deprecated: use BaseModuleContext instead
type BaseContext interface {
	BaseModuleContext
}

type ModuleContext interface {
	BaseModuleContext

	// Deprecated: use ModuleContext.Build instead.
	ModuleBuild(pctx PackageContext, params ModuleBuildParams)

	ExpandSources(srcFiles, excludes []string) Paths
	ExpandSource(srcFile, prop string) Path
	ExpandOptionalSource(srcFile *string, prop string) OptionalPath

	InstallExecutable(installPath InstallPath, name string, srcPath Path, deps ...Path) InstallPath
	InstallFile(installPath InstallPath, name string, srcPath Path, deps ...Path) InstallPath
	InstallSymlink(installPath InstallPath, name string, srcPath InstallPath) InstallPath
	InstallAbsoluteSymlink(installPath InstallPath, name string, absPath string) InstallPath
	CheckbuildFile(srcPath Path)

	InstallInData() bool
	InstallInTestcases() bool
	InstallInSanitizerDir() bool
	InstallInRecovery() bool
	InstallInRoot() bool
	InstallBypassMake() bool

	RequiredModuleNames() []string
	HostRequiredModuleNames() []string
	TargetRequiredModuleNames() []string

	ModuleSubDir() string

	Variable(pctx PackageContext, name, value string)
	Rule(pctx PackageContext, name string, params blueprint.RuleParams, argNames ...string) blueprint.Rule
	// Similar to blueprint.ModuleContext.Build, but takes Paths instead of []string,
	// and performs more verification.
	Build(pctx PackageContext, params BuildParams)

	PrimaryModule() Module
	FinalModule() Module
	VisitAllModuleVariants(visit func(Module))

	GetMissingDependencies() []string
	Namespace() blueprint.Namespace
}

type Module interface {
	blueprint.Module

	// GenerateAndroidBuildActions is analogous to Blueprints' GenerateBuildActions,
	// but GenerateAndroidBuildActions also has access to Android-specific information.
	// For more information, see Module.GenerateBuildActions within Blueprint's module_ctx.go
	GenerateAndroidBuildActions(ModuleContext)

	DepsMutator(BottomUpMutatorContext)

	base() *ModuleBase
	Enabled() bool
	Target() Target
	InstallInData() bool
	InstallInTestcases() bool
	InstallInSanitizerDir() bool
	InstallInRecovery() bool
	InstallInRoot() bool
	InstallBypassMake() bool
	SkipInstall()
	ExportedToMake() bool
	NoticeFile() OptionalPath

	AddProperties(props ...interface{})
	GetProperties() []interface{}

	BuildParamsForTests() []BuildParams
	RuleParamsForTests() map[blueprint.Rule]blueprint.RuleParams
	VariablesForTests() map[string]string

	// String returns a string that includes the module name and variants for printing during debugging.
	String() string

	// Get the qualified module id for this module.
	qualifiedModuleId(ctx BaseModuleContext) qualifiedModuleName

	// Get information about the properties that can contain visibility rules.
	visibilityProperties() []visibilityProperty

	// Get the visibility rules that control the visibility of this module.
	visibility() []string
}

// Qualified id for a module
type qualifiedModuleName struct {
	// The package (i.e. directory) in which the module is defined, without trailing /
	pkg string

	// The name of the module, empty string if package.
	name string
}

func (q qualifiedModuleName) String() string {
	if q.name == "" {
		return "//" + q.pkg
	}
	return "//" + q.pkg + ":" + q.name
}

func (q qualifiedModuleName) isRootPackage() bool {
	return q.pkg == "" && q.name == ""
}

// Get the id for the package containing this module.
func (q qualifiedModuleName) getContainingPackageId() qualifiedModuleName {
	pkg := q.pkg
	if q.name == "" {
		if pkg == "" {
			panic(fmt.Errorf("Cannot get containing package id of root package"))
		}

		index := strings.LastIndex(pkg, "/")
		if index == -1 {
			pkg = ""
		} else {
			pkg = pkg[:index]
		}
	}
	return newPackageId(pkg)
}

func newPackageId(pkg string) qualifiedModuleName {
	// A qualified id for a package module has no name.
	return qualifiedModuleName{pkg: pkg, name: ""}
}

type nameProperties struct {
	// The name of the module.  Must be unique across all modules.
	Name *string
}

type commonProperties struct {
	// emit build rules for this module
	Enabled *bool `android:"arch_variant"`

	// Controls the visibility of this module to other modules. Allowable values are one or more of
	// these formats:
	//
	//  ["//visibility:public"]: Anyone can use this module.
	//  ["//visibility:private"]: Only rules in the module's package (not its subpackages) can use
	//      this module.
	//  ["//some/package:__pkg__", "//other/package:__pkg__"]: Only modules in some/package and
	//      other/package (defined in some/package/*.bp and other/package/*.bp) have access to
	//      this module. Note that sub-packages do not have access to the rule; for example,
	//      //some/package/foo:bar or //other/package/testing:bla wouldn't have access. __pkg__
	//      is a special module and must be used verbatim. It represents all of the modules in the
	//      package.
	//  ["//project:__subpackages__", "//other:__subpackages__"]: Only modules in packages project
	//      or other or in one of their sub-packages have access to this module. For example,
	//      //project:rule, //project/library:lib or //other/testing/internal:munge are allowed
	//      to depend on this rule (but not //independent:evil)
	//  ["//project"]: This is shorthand for ["//project:__pkg__"]
	//  [":__subpackages__"]: This is shorthand for ["//project:__subpackages__"] where
	//      //project is the module's package. e.g. using [":__subpackages__"] in
	//      packages/apps/Settings/Android.bp is equivalent to
	//      //packages/apps/Settings:__subpackages__.
	//  ["//visibility:legacy_public"]: The default visibility, behaves as //visibility:public
	//      for now. It is an error if it is used in a module.
	//
	// If a module does not specify the `visibility` property then it uses the
	// `default_visibility` property of the `package` module in the module's package.
	//
	// If a module does not specify the `visibility` property then it uses the
	// `default_visibility` property of the `package` module in the module's package.
	//
	// If the `default_visibility` property is not set for the module's package then
	// it will use the `default_visibility` of its closest ancestor package for which
	// a `default_visibility` property is specified.
	//
	// If no `default_visibility` property can be found then the module uses the
	// global default of `//visibility:legacy_public`.
	//
	// The `visibility` property has no effect on a defaults module although it does
	// apply to any non-defaults module that uses it. To set the visibility of a
	// defaults module, use the `defaults_visibility` property on the defaults module;
	// not to be confused with the `default_visibility` property on the package module.
	//
	// See https://android.googlesource.com/platform/build/soong/+/master/README.md#visibility for
	// more details.
	Visibility []string

	// control whether this module compiles for 32-bit, 64-bit, or both.  Possible values
	// are "32" (compile for 32-bit only), "64" (compile for 64-bit only), "both" (compile for both
	// architectures), or "first" (compile for 64-bit on a 64-bit platform, and 32-bit on a 32-bit
	// platform
	Compile_multilib *string `android:"arch_variant"`

	Target struct {
		Host struct {
			Compile_multilib *string
		}
		Android struct {
			Compile_multilib *string
		}
	}

	UseTargetVariants bool   `blueprint:"mutated"`
	Default_multilib  string `blueprint:"mutated"`

	// whether this is a proprietary vendor module, and should be installed into /vendor
	Proprietary *bool

	// vendor who owns this module
	Owner *string

	// whether this module is specific to an SoC (System-On-a-Chip). When set to true,
	// it is installed into /vendor (or /system/vendor if vendor partition does not exist).
	// Use `soc_specific` instead for better meaning.
	Vendor *bool

	// whether this module is specific to an SoC (System-On-a-Chip). When set to true,
	// it is installed into /vendor (or /system/vendor if vendor partition does not exist).
	Soc_specific *bool

	// whether this module is specific to a device, not only for SoC, but also for off-chip
	// peripherals. When set to true, it is installed into /odm (or /vendor/odm if odm partition
	// does not exist, or /system/vendor/odm if both odm and vendor partitions do not exist).
	// This implies `soc_specific:true`.
	Device_specific *bool

	// whether this module is specific to a software configuration of a product (e.g. country,
	// network operator, etc). When set to true, it is installed into /product (or
	// /system/product if product partition does not exist).
	Product_specific *bool

	// TODO(b/135957588) Product_services_specific will be removed once we clear all Android.bp
	// files that have 'product_services_specific: true'. This will be converted to
	// Product_speicific as a workaround.
	Product_services_specific *bool

	// whether this module extends system. When set to true, it is installed into /system_ext
	// (or /system/system_ext if system_ext partition does not exist).
	System_ext_specific *bool

	// Whether this module is installed to recovery partition
	Recovery *bool

	// Whether this module is built for non-native architecures (also known as native bridge binary)
	Native_bridge_supported *bool `android:"arch_variant"`

	// init.rc files to be installed if this module is installed
	Init_rc []string `android:"path"`

	// VINTF manifest fragments to be installed if this module is installed
	Vintf_fragments []string `android:"path"`

	// names of other modules to install if this module is installed
	Required []string `android:"arch_variant"`

	// names of other modules to install on host if this module is installed
	Host_required []string `android:"arch_variant"`

	// names of other modules to install on target if this module is installed
	Target_required []string `android:"arch_variant"`

	// relative path to a file to include in the list of notices for the device
	Notice *string `android:"path"`

	Dist struct {
		// copy the output of this module to the $DIST_DIR when `dist` is specified on the
		// command line and  any of these targets are also on the command line, or otherwise
		// built
		Targets []string `android:"arch_variant"`

		// The name of the output artifact. This defaults to the basename of the output of
		// the module.
		Dest *string `android:"arch_variant"`

		// The directory within the dist directory to store the artifact. Defaults to the
		// top level directory ("").
		Dir *string `android:"arch_variant"`

		// A suffix to add to the artifact file name (before any extension).
		Suffix *string `android:"arch_variant"`
	} `android:"arch_variant"`

	// Set by TargetMutator
	CompileOS           OsType   `blueprint:"mutated"`
	CompileTarget       Target   `blueprint:"mutated"`
	CompileMultiTargets []Target `blueprint:"mutated"`
	CompilePrimary      bool     `blueprint:"mutated"`

	// Set by InitAndroidModule
	HostOrDeviceSupported HostOrDeviceSupported `blueprint:"mutated"`
	ArchSpecific          bool                  `blueprint:"mutated"`

	SkipInstall bool `blueprint:"mutated"`

	NamespaceExportedToMake bool `blueprint:"mutated"`

	MissingDeps []string `blueprint:"mutated"`

	// Name and variant strings stored by mutators to enable Module.String()
	DebugName       string   `blueprint:"mutated"`
	DebugMutators   []string `blueprint:"mutated"`
	DebugVariations []string `blueprint:"mutated"`
}

type hostAndDeviceProperties struct {
	// If set to true, build a variant of the module for the host.  Defaults to false.
	Host_supported *bool

	// If set to true, build a variant of the module for the device.  Defaults to true.
	Device_supported *bool
}

type Multilib string

const (
	MultilibBoth        Multilib = "both"
	MultilibFirst       Multilib = "first"
	MultilibCommon      Multilib = "common"
	MultilibCommonFirst Multilib = "common_first"
	MultilibDefault     Multilib = ""
)

type HostOrDeviceSupported int

const (
	_ HostOrDeviceSupported = iota

	// Host and HostCross are built by default. Device is not supported.
	HostSupported

	// Host is built by default. HostCross and Device are not supported.
	HostSupportedNoCross

	// Device is built by default. Host and HostCross are not supported.
	DeviceSupported

	// Device is built by default. Host and HostCross are supported.
	HostAndDeviceSupported

	// Host, HostCross, and Device are built by default.
	HostAndDeviceDefault

	// Nothing is supported. This is not exposed to the user, but used to mark a
	// host only module as unsupported when the module type is not supported on
	// the host OS. E.g. benchmarks are supported on Linux but not Darwin.
	NeitherHostNorDeviceSupported
)

type moduleKind int

const (
	platformModule moduleKind = iota
	deviceSpecificModule
	socSpecificModule
	productSpecificModule
	systemExtSpecificModule
)

func (k moduleKind) String() string {
	switch k {
	case platformModule:
		return "platform"
	case deviceSpecificModule:
		return "device-specific"
	case socSpecificModule:
		return "soc-specific"
	case productSpecificModule:
		return "product-specific"
	case systemExtSpecificModule:
		return "systemext-specific"
	default:
		panic(fmt.Errorf("unknown module kind %d", k))
	}
}

func InitAndroidModule(m Module) {
	base := m.base()
	base.module = m

	m.AddProperties(
		&base.nameProperties,
		&base.commonProperties)

	// Allow tests to override the default product variables
	if base.variableProperties == nil {
		base.variableProperties = zeroProductVariables
	}

	// Filter the product variables properties to the ones that exist on this module
	base.variableProperties = createVariableProperties(m.GetProperties(), base.variableProperties)
	if base.variableProperties != nil {
		m.AddProperties(base.variableProperties)
	}

	base.generalProperties = m.GetProperties()
	base.customizableProperties = m.GetProperties()

	// The default_visibility property needs to be checked and parsed by the visibility module during
	// its checking and parsing phases.
	base.primaryVisibilityProperty =
		newVisibilityProperty("visibility", &base.commonProperties.Visibility)
	base.visibilityPropertyInfo = []visibilityProperty{base.primaryVisibilityProperty}
}

func InitAndroidArchModule(m Module, hod HostOrDeviceSupported, defaultMultilib Multilib) {
	InitAndroidModule(m)

	base := m.base()
	base.commonProperties.HostOrDeviceSupported = hod
	base.commonProperties.Default_multilib = string(defaultMultilib)
	base.commonProperties.ArchSpecific = true
	base.commonProperties.UseTargetVariants = true

	switch hod {
	case HostAndDeviceSupported, HostAndDeviceDefault:
		m.AddProperties(&base.hostAndDeviceProperties)
	}

	InitArchModule(m)
}

func InitAndroidMultiTargetsArchModule(m Module, hod HostOrDeviceSupported, defaultMultilib Multilib) {
	InitAndroidArchModule(m, hod, defaultMultilib)
	m.base().commonProperties.UseTargetVariants = false
}

// A ModuleBase object contains the properties that are common to all Android
// modules.  It should be included as an anonymous field in every module
// struct definition.  InitAndroidModule should then be called from the module's
// factory function, and the return values from InitAndroidModule should be
// returned from the factory function.
//
// The ModuleBase type is responsible for implementing the GenerateBuildActions
// method to support the blueprint.Module interface. This method will then call
// the module's GenerateAndroidBuildActions method once for each build variant
// that is to be built. GenerateAndroidBuildActions is passed a ModuleContext
// rather than the usual blueprint.ModuleContext.
// ModuleContext exposes extra functionality specific to the Android build
// system including details about the particular build variant that is to be
// generated.
//
// For example:
//
//     import (
//         "android/soong/android"
//     )
//
//     type myModule struct {
//         android.ModuleBase
//         properties struct {
//             MyProperty string
//         }
//     }
//
//     func NewMyModule() android.Module) {
//         m := &myModule{}
//         m.AddProperties(&m.properties)
//         android.InitAndroidModule(m)
//         return m
//     }
//
//     func (m *myModule) GenerateAndroidBuildActions(ctx android.ModuleContext) {
//         // Get the CPU architecture for the current build variant.
//         variantArch := ctx.Arch()
//
//         // ...
//     }
type ModuleBase struct {
	// Putting the curiously recurring thing pointing to the thing that contains
	// the thing pattern to good use.
	// TODO: remove this
	module Module

	nameProperties          nameProperties
	commonProperties        commonProperties
	variableProperties      interface{}
	hostAndDeviceProperties hostAndDeviceProperties
	generalProperties       []interface{}
	archProperties          [][]interface{}
	customizableProperties  []interface{}

	// Information about all the properties on the module that contains visibility rules that need
	// checking.
	visibilityPropertyInfo []visibilityProperty

	// The primary visibility property, may be nil, that controls access to the module.
	primaryVisibilityProperty visibilityProperty

	noAddressSanitizer bool
	installFiles       Paths
	checkbuildFiles    Paths
	noticeFile         OptionalPath

	// Used by buildTargetSingleton to create checkbuild and per-directory build targets
	// Only set on the final variant of each module
	installTarget    WritablePath
	checkbuildTarget WritablePath
	blueprintDir     string

	hooks hooks

	registerProps []interface{}

	// For tests
	buildParams []BuildParams
	ruleParams  map[blueprint.Rule]blueprint.RuleParams
	variables   map[string]string

	prefer32 func(ctx BaseModuleContext, base *ModuleBase, class OsClass) bool
}

func (m *ModuleBase) DepsMutator(BottomUpMutatorContext) {}

func (m *ModuleBase) AddProperties(props ...interface{}) {
	m.registerProps = append(m.registerProps, props...)
}

func (m *ModuleBase) GetProperties() []interface{} {
	return m.registerProps
}

func (m *ModuleBase) BuildParamsForTests() []BuildParams {
	return m.buildParams
}

func (m *ModuleBase) RuleParamsForTests() map[blueprint.Rule]blueprint.RuleParams {
	return m.ruleParams
}

func (m *ModuleBase) VariablesForTests() map[string]string {
	return m.variables
}

func (m *ModuleBase) Prefer32(prefer32 func(ctx BaseModuleContext, base *ModuleBase, class OsClass) bool) {
	m.prefer32 = prefer32
}

// Name returns the name of the module.  It may be overridden by individual module types, for
// example prebuilts will prepend prebuilt_ to the name.
func (m *ModuleBase) Name() string {
	return String(m.nameProperties.Name)
}

// String returns a string that includes the module name and variants for printing during debugging.
func (m *ModuleBase) String() string {
	sb := strings.Builder{}
	sb.WriteString(m.commonProperties.DebugName)
	sb.WriteString("{")
	for i := range m.commonProperties.DebugMutators {
		if i != 0 {
			sb.WriteString(",")
		}
		sb.WriteString(m.commonProperties.DebugMutators[i])
		sb.WriteString(":")
		sb.WriteString(m.commonProperties.DebugVariations[i])
	}
	sb.WriteString("}")
	return sb.String()
}

// BaseModuleName returns the name of the module as specified in the blueprints file.
func (m *ModuleBase) BaseModuleName() string {
	return String(m.nameProperties.Name)
}

func (m *ModuleBase) base() *ModuleBase {
	return m
}

func (m *ModuleBase) qualifiedModuleId(ctx BaseModuleContext) qualifiedModuleName {
	return qualifiedModuleName{pkg: ctx.ModuleDir(), name: ctx.ModuleName()}
}

func (m *ModuleBase) visibilityProperties() []visibilityProperty {
	return m.visibilityPropertyInfo
}

func (m *ModuleBase) visibility() []string {
	// The soong_namespace module does not initialize the primaryVisibilityProperty.
	if m.primaryVisibilityProperty != nil {
		return m.primaryVisibilityProperty.getStrings()
	} else {
		return nil
	}
}

func (m *ModuleBase) Target() Target {
	return m.commonProperties.CompileTarget
}

func (m *ModuleBase) TargetPrimary() bool {
	return m.commonProperties.CompilePrimary
}

func (m *ModuleBase) MultiTargets() []Target {
	return m.commonProperties.CompileMultiTargets
}

func (m *ModuleBase) Os() OsType {
	return m.Target().Os
}

func (m *ModuleBase) Host() bool {
	return m.Os().Class == Host || m.Os().Class == HostCross
}

func (m *ModuleBase) Arch() Arch {
	return m.Target().Arch
}

func (m *ModuleBase) ArchSpecific() bool {
	return m.commonProperties.ArchSpecific
}

func (m *ModuleBase) OsClassSupported() []OsClass {
	switch m.commonProperties.HostOrDeviceSupported {
	case HostSupported:
		return []OsClass{Host, HostCross}
	case HostSupportedNoCross:
		return []OsClass{Host}
	case DeviceSupported:
		return []OsClass{Device}
	case HostAndDeviceSupported, HostAndDeviceDefault:
		var supported []OsClass
		if Bool(m.hostAndDeviceProperties.Host_supported) ||
			(m.commonProperties.HostOrDeviceSupported == HostAndDeviceDefault &&
				m.hostAndDeviceProperties.Host_supported == nil) {
			supported = append(supported, Host, HostCross)
		}
		if m.hostAndDeviceProperties.Device_supported == nil ||
			*m.hostAndDeviceProperties.Device_supported {
			supported = append(supported, Device)
		}
		return supported
	default:
		return nil
	}
}

func (m *ModuleBase) DeviceSupported() bool {
	return m.commonProperties.HostOrDeviceSupported == DeviceSupported ||
		m.commonProperties.HostOrDeviceSupported == HostAndDeviceSupported &&
			(m.hostAndDeviceProperties.Device_supported == nil ||
				*m.hostAndDeviceProperties.Device_supported)
}

func (m *ModuleBase) Platform() bool {
	return !m.DeviceSpecific() && !m.SocSpecific() && !m.ProductSpecific() && !m.SystemExtSpecific()
}

func (m *ModuleBase) DeviceSpecific() bool {
	return Bool(m.commonProperties.Device_specific)
}

func (m *ModuleBase) SocSpecific() bool {
	return Bool(m.commonProperties.Vendor) || Bool(m.commonProperties.Proprietary) || Bool(m.commonProperties.Soc_specific)
}

func (m *ModuleBase) ProductSpecific() bool {
	return Bool(m.commonProperties.Product_specific)
}

func (m *ModuleBase) SystemExtSpecific() bool {
	return Bool(m.commonProperties.System_ext_specific)
}

func (m *ModuleBase) Enabled() bool {
	if m.commonProperties.Enabled == nil {
		return !m.Os().DefaultDisabled
	}
	return *m.commonProperties.Enabled
}

func (m *ModuleBase) SkipInstall() {
	m.commonProperties.SkipInstall = true
}

func (m *ModuleBase) ExportedToMake() bool {
	return m.commonProperties.NamespaceExportedToMake
}

func (m *ModuleBase) computeInstallDeps(
	ctx blueprint.ModuleContext) Paths {

	result := Paths{}
	// TODO(ccross): we need to use WalkDeps and have some way to know which dependencies require installation
	ctx.VisitDepsDepthFirstIf(isFileInstaller,
		func(m blueprint.Module) {
			fileInstaller := m.(fileInstaller)
			files := fileInstaller.filesToInstall()
			result = append(result, files...)
		})

	return result
}

func (m *ModuleBase) filesToInstall() Paths {
	return m.installFiles
}

func (m *ModuleBase) NoAddressSanitizer() bool {
	return m.noAddressSanitizer
}

func (m *ModuleBase) InstallInData() bool {
	return false
}

func (m *ModuleBase) InstallInTestcases() bool {
	return false
}

func (m *ModuleBase) InstallInSanitizerDir() bool {
	return false
}

func (m *ModuleBase) InstallInRecovery() bool {
	return Bool(m.commonProperties.Recovery)
}

func (m *ModuleBase) InstallInRoot() bool {
	return false
}

func (m *ModuleBase) InstallBypassMake() bool {
	return false
}

func (m *ModuleBase) Owner() string {
	return String(m.commonProperties.Owner)
}

func (m *ModuleBase) NoticeFile() OptionalPath {
	return m.noticeFile
}

func (m *ModuleBase) generateModuleTarget(ctx ModuleContext) {
	allInstalledFiles := Paths{}
	allCheckbuildFiles := Paths{}
	ctx.VisitAllModuleVariants(func(module Module) {
		a := module.base()
		allInstalledFiles = append(allInstalledFiles, a.installFiles...)
		allCheckbuildFiles = append(allCheckbuildFiles, a.checkbuildFiles...)
	})

	var deps Paths

	namespacePrefix := ctx.Namespace().(*Namespace).id
	if namespacePrefix != "" {
		namespacePrefix = namespacePrefix + "-"
	}

	if len(allInstalledFiles) > 0 {
		name := PathForPhony(ctx, namespacePrefix+ctx.ModuleName()+"-install")
		ctx.Build(pctx, BuildParams{
			Rule:      blueprint.Phony,
			Output:    name,
			Implicits: allInstalledFiles,
			Default:   !ctx.Config().EmbeddedInMake(),
		})
		deps = append(deps, name)
		m.installTarget = name
	}

	if len(allCheckbuildFiles) > 0 {
		name := PathForPhony(ctx, namespacePrefix+ctx.ModuleName()+"-checkbuild")
		ctx.Build(pctx, BuildParams{
			Rule:      blueprint.Phony,
			Output:    name,
			Implicits: allCheckbuildFiles,
		})
		deps = append(deps, name)
		m.checkbuildTarget = name
	}

	if len(deps) > 0 {
		suffix := ""
		if ctx.Config().EmbeddedInMake() {
			suffix = "-soong"
		}

		name := PathForPhony(ctx, namespacePrefix+ctx.ModuleName()+suffix)
		ctx.Build(pctx, BuildParams{
			Rule:      blueprint.Phony,
			Outputs:   []WritablePath{name},
			Implicits: deps,
		})

		m.blueprintDir = ctx.ModuleDir()
	}
}

func determineModuleKind(m *ModuleBase, ctx blueprint.BaseModuleContext) moduleKind {
	var socSpecific = Bool(m.commonProperties.Vendor) || Bool(m.commonProperties.Proprietary) || Bool(m.commonProperties.Soc_specific)
	var deviceSpecific = Bool(m.commonProperties.Device_specific)
	var productSpecific = Bool(m.commonProperties.Product_specific)
	var systemExtSpecific = Bool(m.commonProperties.System_ext_specific)

	msg := "conflicting value set here"
	if socSpecific && deviceSpecific {
		ctx.PropertyErrorf("device_specific", "a module cannot be specific to SoC and device at the same time.")
		if Bool(m.commonProperties.Vendor) {
			ctx.PropertyErrorf("vendor", msg)
		}
		if Bool(m.commonProperties.Proprietary) {
			ctx.PropertyErrorf("proprietary", msg)
		}
		if Bool(m.commonProperties.Soc_specific) {
			ctx.PropertyErrorf("soc_specific", msg)
		}
	}

	if productSpecific && systemExtSpecific {
		ctx.PropertyErrorf("product_specific", "a module cannot be specific to product and system_ext at the same time.")
		ctx.PropertyErrorf("system_ext_specific", msg)
	}

	if (socSpecific || deviceSpecific) && (productSpecific || systemExtSpecific) {
		if productSpecific {
			ctx.PropertyErrorf("product_specific", "a module cannot be specific to SoC or device and product at the same time.")
		} else {
			ctx.PropertyErrorf("system_ext_specific", "a module cannot be specific to SoC or device and system_ext at the same time.")
		}
		if deviceSpecific {
			ctx.PropertyErrorf("device_specific", msg)
		} else {
			if Bool(m.commonProperties.Vendor) {
				ctx.PropertyErrorf("vendor", msg)
			}
			if Bool(m.commonProperties.Proprietary) {
				ctx.PropertyErrorf("proprietary", msg)
			}
			if Bool(m.commonProperties.Soc_specific) {
				ctx.PropertyErrorf("soc_specific", msg)
			}
		}
	}

	if productSpecific {
		return productSpecificModule
	} else if systemExtSpecific {
		return systemExtSpecificModule
	} else if deviceSpecific {
		return deviceSpecificModule
	} else if socSpecific {
		return socSpecificModule
	} else {
		return platformModule
	}
}

func (m *ModuleBase) baseModuleContextFactory(ctx blueprint.BaseModuleContext) baseModuleContext {
	return baseModuleContext{
		BaseModuleContext: ctx,
		target:            m.commonProperties.CompileTarget,
		targetPrimary:     m.commonProperties.CompilePrimary,
		multiTargets:      m.commonProperties.CompileMultiTargets,
		kind:              determineModuleKind(m, ctx),
		config:            ctx.Config().(Config),
	}
}

func (m *ModuleBase) GenerateBuildActions(blueprintCtx blueprint.ModuleContext) {
	ctx := &moduleContext{
		module:            m.module,
		bp:                blueprintCtx,
		baseModuleContext: m.baseModuleContextFactory(blueprintCtx),
		installDeps:       m.computeInstallDeps(blueprintCtx),
		installFiles:      m.installFiles,
		variables:         make(map[string]string),
	}

	// Temporarily continue to call blueprintCtx.GetMissingDependencies() to maintain the previous behavior of never
	// reporting missing dependency errors in Blueprint when AllowMissingDependencies == true.
	// TODO: This will be removed once defaults modules handle missing dependency errors
	blueprintCtx.GetMissingDependencies()

	// For the final GenerateAndroidBuildActions pass, require that all visited dependencies Soong modules and
	// are enabled.
	ctx.baseModuleContext.strictVisitDeps = true

	if ctx.config.captureBuild {
		ctx.ruleParams = make(map[blueprint.Rule]blueprint.RuleParams)
	}

	desc := "//" + ctx.ModuleDir() + ":" + ctx.ModuleName() + " "
	var suffix []string
	if ctx.Os().Class != Device && ctx.Os().Class != Generic {
		suffix = append(suffix, ctx.Os().String())
	}
	if !ctx.PrimaryArch() {
		suffix = append(suffix, ctx.Arch().ArchType.String())
	}

	ctx.Variable(pctx, "moduleDesc", desc)

	s := ""
	if len(suffix) > 0 {
		s = " [" + strings.Join(suffix, " ") + "]"
	}
	ctx.Variable(pctx, "moduleDescSuffix", s)

	// Some common property checks for properties that will be used later in androidmk.go
	if m.commonProperties.Dist.Dest != nil {
		_, err := validateSafePath(*m.commonProperties.Dist.Dest)
		if err != nil {
			ctx.PropertyErrorf("dist.dest", "%s", err.Error())
		}
	}
	if m.commonProperties.Dist.Dir != nil {
		_, err := validateSafePath(*m.commonProperties.Dist.Dir)
		if err != nil {
			ctx.PropertyErrorf("dist.dir", "%s", err.Error())
		}
	}
	if m.commonProperties.Dist.Suffix != nil {
		if strings.Contains(*m.commonProperties.Dist.Suffix, "/") {
			ctx.PropertyErrorf("dist.suffix", "Suffix may not contain a '/' character.")
		}
	}

	if m.Enabled() {
		// ensure all direct android.Module deps are enabled
		ctx.VisitDirectDepsBlueprint(func(bm blueprint.Module) {
			if _, ok := bm.(Module); ok {
				ctx.validateAndroidModule(bm, ctx.baseModuleContext.strictVisitDeps)
			}
		})

		notice := proptools.StringDefault(m.commonProperties.Notice, "NOTICE")
		if module := SrcIsModule(notice); module != "" {
			m.noticeFile = ctx.ExpandOptionalSource(&notice, "notice")
		} else {
			noticePath := filepath.Join(ctx.ModuleDir(), notice)
			m.noticeFile = ExistentPathForSource(ctx, noticePath)
		}

		m.module.GenerateAndroidBuildActions(ctx)
		if ctx.Failed() {
			return
		}

		m.installFiles = append(m.installFiles, ctx.installFiles...)
		m.checkbuildFiles = append(m.checkbuildFiles, ctx.checkbuildFiles...)
	} else if ctx.Config().AllowMissingDependencies() {
		// If the module is not enabled it will not create any build rules, nothing will call
		// ctx.GetMissingDependencies(), and blueprint will consider the missing dependencies to be unhandled
		// and report them as an error even when AllowMissingDependencies = true.  Call
		// ctx.GetMissingDependencies() here to tell blueprint not to handle them.
		ctx.GetMissingDependencies()
	}

	if m == ctx.FinalModule().(Module).base() {
		m.generateModuleTarget(ctx)
		if ctx.Failed() {
			return
		}
	}

	m.buildParams = ctx.buildParams
	m.ruleParams = ctx.ruleParams
	m.variables = ctx.variables
}

type baseModuleContext struct {
	blueprint.BaseModuleContext
	target        Target
	multiTargets  []Target
	targetPrimary bool
	debug         bool
	kind          moduleKind
	config        Config

	walkPath []Module

	strictVisitDeps bool // If true, enforce that all dependencies are enabled
}

type moduleContext struct {
	bp blueprint.ModuleContext
	baseModuleContext
	installDeps     Paths
	installFiles    Paths
	checkbuildFiles Paths
	module          Module

	// For tests
	buildParams []BuildParams
	ruleParams  map[blueprint.Rule]blueprint.RuleParams
	variables   map[string]string
}

func (m *moduleContext) ninjaError(params BuildParams, err error) (PackageContext, BuildParams) {
	return pctx, BuildParams{
		Rule:            ErrorRule,
		Description:     params.Description,
		Output:          params.Output,
		Outputs:         params.Outputs,
		ImplicitOutput:  params.ImplicitOutput,
		ImplicitOutputs: params.ImplicitOutputs,
		Args: map[string]string{
			"error": err.Error(),
		},
	}
}

func (m *moduleContext) ModuleBuild(pctx PackageContext, params ModuleBuildParams) {
	m.Build(pctx, BuildParams(params))
}

func convertBuildParams(params BuildParams) blueprint.BuildParams {
	bparams := blueprint.BuildParams{
		Rule:            params.Rule,
		Description:     params.Description,
		Deps:            params.Deps,
		Outputs:         params.Outputs.Strings(),
		ImplicitOutputs: params.ImplicitOutputs.Strings(),
		Inputs:          params.Inputs.Strings(),
		Implicits:       params.Implicits.Strings(),
		OrderOnly:       params.OrderOnly.Strings(),
		Args:            params.Args,
		Optional:        !params.Default,
	}

	if params.Depfile != nil {
		bparams.Depfile = params.Depfile.String()
	}
	if params.Output != nil {
		bparams.Outputs = append(bparams.Outputs, params.Output.String())
	}
	if params.ImplicitOutput != nil {
		bparams.ImplicitOutputs = append(bparams.ImplicitOutputs, params.ImplicitOutput.String())
	}
	if params.Input != nil {
		bparams.Inputs = append(bparams.Inputs, params.Input.String())
	}
	if params.Implicit != nil {
		bparams.Implicits = append(bparams.Implicits, params.Implicit.String())
	}

	bparams.Outputs = proptools.NinjaEscapeList(bparams.Outputs)
	bparams.ImplicitOutputs = proptools.NinjaEscapeList(bparams.ImplicitOutputs)
	bparams.Inputs = proptools.NinjaEscapeList(bparams.Inputs)
	bparams.Implicits = proptools.NinjaEscapeList(bparams.Implicits)
	bparams.OrderOnly = proptools.NinjaEscapeList(bparams.OrderOnly)
	bparams.Depfile = proptools.NinjaEscapeList([]string{bparams.Depfile})[0]

	return bparams
}

func (m *moduleContext) Variable(pctx PackageContext, name, value string) {
	if m.config.captureBuild {
		m.variables[name] = value
	}

	m.bp.Variable(pctx.PackageContext, name, value)
}

func (m *moduleContext) Rule(pctx PackageContext, name string, params blueprint.RuleParams,
	argNames ...string) blueprint.Rule {

	if m.config.UseGoma() && params.Pool == nil {
		// When USE_GOMA=true is set and the rule is not supported by goma, restrict jobs to the
		// local parallelism value
		params.Pool = localPool
	}

	rule := m.bp.Rule(pctx.PackageContext, name, params, argNames...)

	if m.config.captureBuild {
		m.ruleParams[rule] = params
	}

	return rule
}

func (m *moduleContext) Build(pctx PackageContext, params BuildParams) {
	if params.Description != "" {
		params.Description = "${moduleDesc}" + params.Description + "${moduleDescSuffix}"
	}

	if missingDeps := m.GetMissingDependencies(); len(missingDeps) > 0 {
		pctx, params = m.ninjaError(params, fmt.Errorf("module %s missing dependencies: %s\n",
			m.ModuleName(), strings.Join(missingDeps, ", ")))
	}

	if m.config.captureBuild {
		m.buildParams = append(m.buildParams, params)
	}

	m.bp.Build(pctx.PackageContext, convertBuildParams(params))
}

func (b *baseModuleContext) Module() Module {
	module, _ := b.BaseModuleContext.Module().(Module)
	return module
}

func (b *baseModuleContext) Config() Config {
	return b.BaseModuleContext.Config().(Config)
}

func (m *moduleContext) GetMissingDependencies() []string {
	var missingDeps []string
	missingDeps = append(missingDeps, m.Module().base().commonProperties.MissingDeps...)
	missingDeps = append(missingDeps, m.bp.GetMissingDependencies()...)
	missingDeps = FirstUniqueStrings(missingDeps)
	return missingDeps
}

func (b *baseModuleContext) AddMissingDependencies(deps []string) {
	if deps != nil {
		missingDeps := &b.Module().base().commonProperties.MissingDeps
		*missingDeps = append(*missingDeps, deps...)
		*missingDeps = FirstUniqueStrings(*missingDeps)
	}
}

func (b *baseModuleContext) validateAndroidModule(module blueprint.Module, strict bool) Module {
	aModule, _ := module.(Module)

	if !strict {
		return aModule
	}

	if aModule == nil {
		b.ModuleErrorf("module %q not an android module", b.OtherModuleName(module))
		return nil
	}

	if !aModule.Enabled() {
		if b.Config().AllowMissingDependencies() {
			b.AddMissingDependencies([]string{b.OtherModuleName(aModule)})
		} else {
			b.ModuleErrorf("depends on disabled module %q", b.OtherModuleName(aModule))
		}
		return nil
	}
	return aModule
}

func (b *baseModuleContext) getDirectDepInternal(name string, tag blueprint.DependencyTag) (blueprint.Module, blueprint.DependencyTag) {
	type dep struct {
		mod blueprint.Module
		tag blueprint.DependencyTag
	}
	var deps []dep
	b.VisitDirectDepsBlueprint(func(module blueprint.Module) {
		if aModule, _ := module.(Module); aModule != nil && aModule.base().BaseModuleName() == name {
			returnedTag := b.BaseModuleContext.OtherModuleDependencyTag(aModule)
			if tag == nil || returnedTag == tag {
				deps = append(deps, dep{aModule, returnedTag})
			}
		}
	})
	if len(deps) == 1 {
		return deps[0].mod, deps[0].tag
	} else if len(deps) >= 2 {
		panic(fmt.Errorf("Multiple dependencies having same BaseModuleName() %q found from %q",
			name, b.ModuleName()))
	} else {
		return nil, nil
	}
}

func (b *baseModuleContext) GetDirectDepsWithTag(tag blueprint.DependencyTag) []Module {
	var deps []Module
	b.VisitDirectDepsBlueprint(func(module blueprint.Module) {
		if aModule, _ := module.(Module); aModule != nil {
			if b.BaseModuleContext.OtherModuleDependencyTag(aModule) == tag {
				deps = append(deps, aModule)
			}
		}
	})
	return deps
}

func (m *moduleContext) GetDirectDepWithTag(name string, tag blueprint.DependencyTag) blueprint.Module {
	module, _ := m.getDirectDepInternal(name, tag)
	return module
}

func (b *baseModuleContext) GetDirectDep(name string) (blueprint.Module, blueprint.DependencyTag) {
	return b.getDirectDepInternal(name, nil)
}

func (b *baseModuleContext) VisitDirectDepsBlueprint(visit func(blueprint.Module)) {
	b.BaseModuleContext.VisitDirectDeps(visit)
}

func (b *baseModuleContext) VisitDirectDeps(visit func(Module)) {
	b.BaseModuleContext.VisitDirectDeps(func(module blueprint.Module) {
		if aModule := b.validateAndroidModule(module, b.strictVisitDeps); aModule != nil {
			visit(aModule)
		}
	})
}

func (b *baseModuleContext) VisitDirectDepsWithTag(tag blueprint.DependencyTag, visit func(Module)) {
	b.BaseModuleContext.VisitDirectDeps(func(module blueprint.Module) {
		if aModule := b.validateAndroidModule(module, b.strictVisitDeps); aModule != nil {
			if b.BaseModuleContext.OtherModuleDependencyTag(aModule) == tag {
				visit(aModule)
			}
		}
	})
}

func (b *baseModuleContext) VisitDirectDepsIf(pred func(Module) bool, visit func(Module)) {
	b.BaseModuleContext.VisitDirectDepsIf(
		// pred
		func(module blueprint.Module) bool {
			if aModule := b.validateAndroidModule(module, b.strictVisitDeps); aModule != nil {
				return pred(aModule)
			} else {
				return false
			}
		},
		// visit
		func(module blueprint.Module) {
			visit(module.(Module))
		})
}

func (b *baseModuleContext) VisitDepsDepthFirst(visit func(Module)) {
	b.BaseModuleContext.VisitDepsDepthFirst(func(module blueprint.Module) {
		if aModule := b.validateAndroidModule(module, b.strictVisitDeps); aModule != nil {
			visit(aModule)
		}
	})
}

func (b *baseModuleContext) VisitDepsDepthFirstIf(pred func(Module) bool, visit func(Module)) {
	b.BaseModuleContext.VisitDepsDepthFirstIf(
		// pred
		func(module blueprint.Module) bool {
			if aModule := b.validateAndroidModule(module, b.strictVisitDeps); aModule != nil {
				return pred(aModule)
			} else {
				return false
			}
		},
		// visit
		func(module blueprint.Module) {
			visit(module.(Module))
		})
}

func (b *baseModuleContext) WalkDepsBlueprint(visit func(blueprint.Module, blueprint.Module) bool) {
	b.BaseModuleContext.WalkDeps(visit)
}

func (b *baseModuleContext) WalkDeps(visit func(Module, Module) bool) {
	b.walkPath = []Module{b.Module()}
	b.BaseModuleContext.WalkDeps(func(child, parent blueprint.Module) bool {
		childAndroidModule, _ := child.(Module)
		parentAndroidModule, _ := parent.(Module)
		if childAndroidModule != nil && parentAndroidModule != nil {
			// record walkPath before visit
			for b.walkPath[len(b.walkPath)-1] != parentAndroidModule {
				b.walkPath = b.walkPath[0 : len(b.walkPath)-1]
			}
			b.walkPath = append(b.walkPath, childAndroidModule)
			return visit(childAndroidModule, parentAndroidModule)
		} else {
			return false
		}
	})
}

func (b *baseModuleContext) GetWalkPath() []Module {
	return b.walkPath
}

func (m *moduleContext) VisitAllModuleVariants(visit func(Module)) {
	m.bp.VisitAllModuleVariants(func(module blueprint.Module) {
		visit(module.(Module))
	})
}

func (m *moduleContext) PrimaryModule() Module {
	return m.bp.PrimaryModule().(Module)
}

func (m *moduleContext) FinalModule() Module {
	return m.bp.FinalModule().(Module)
}

func (m *moduleContext) ModuleSubDir() string {
	return m.bp.ModuleSubDir()
}

func (b *baseModuleContext) Target() Target {
	return b.target
}

func (b *baseModuleContext) TargetPrimary() bool {
	return b.targetPrimary
}

func (b *baseModuleContext) MultiTargets() []Target {
	return b.multiTargets
}

func (b *baseModuleContext) Arch() Arch {
	return b.target.Arch
}

func (b *baseModuleContext) Os() OsType {
	return b.target.Os
}

func (b *baseModuleContext) Host() bool {
	return b.target.Os.Class == Host || b.target.Os.Class == HostCross
}

func (b *baseModuleContext) Device() bool {
	return b.target.Os.Class == Device
}

func (b *baseModuleContext) Darwin() bool {
	return b.target.Os == Darwin
}

func (b *baseModuleContext) Fuchsia() bool {
	return b.target.Os == Fuchsia
}

func (b *baseModuleContext) Windows() bool {
	return b.target.Os == Windows
}

func (b *baseModuleContext) Debug() bool {
	return b.debug
}

func (b *baseModuleContext) PrimaryArch() bool {
	if len(b.config.Targets[b.target.Os]) <= 1 {
		return true
	}
	return b.target.Arch.ArchType == b.config.Targets[b.target.Os][0].Arch.ArchType
}

func (b *baseModuleContext) AConfig() Config {
	return b.config
}

func (b *baseModuleContext) DeviceConfig() DeviceConfig {
	return DeviceConfig{b.config.deviceConfig}
}

func (b *baseModuleContext) Platform() bool {
	return b.kind == platformModule
}

func (b *baseModuleContext) DeviceSpecific() bool {
	return b.kind == deviceSpecificModule
}

func (b *baseModuleContext) SocSpecific() bool {
	return b.kind == socSpecificModule
}

func (b *baseModuleContext) ProductSpecific() bool {
	return b.kind == productSpecificModule
}

func (b *baseModuleContext) SystemExtSpecific() bool {
	return b.kind == systemExtSpecificModule
}

// Makes this module a platform module, i.e. not specific to soc, device,
// product, or system_ext.
func (m *ModuleBase) MakeAsPlatform() {
	m.commonProperties.Vendor = boolPtr(false)
	m.commonProperties.Proprietary = boolPtr(false)
	m.commonProperties.Soc_specific = boolPtr(false)
	m.commonProperties.Product_specific = boolPtr(false)
	m.commonProperties.System_ext_specific = boolPtr(false)
}

func (m *ModuleBase) EnableNativeBridgeSupportByDefault() {
	m.commonProperties.Native_bridge_supported = boolPtr(true)
}

// IsNativeBridgeSupported returns true if "native_bridge_supported" is explicitly set as "true"
func (m *ModuleBase) IsNativeBridgeSupported() bool {
	return proptools.Bool(m.commonProperties.Native_bridge_supported)
}

func (m *moduleContext) InstallInData() bool {
	return m.module.InstallInData()
}

func (m *moduleContext) InstallInTestcases() bool {
	return m.module.InstallInTestcases()
}

func (m *moduleContext) InstallInSanitizerDir() bool {
	return m.module.InstallInSanitizerDir()
}

func (m *moduleContext) InstallInRecovery() bool {
	return m.module.InstallInRecovery()
}

func (m *moduleContext) InstallInRoot() bool {
	return m.module.InstallInRoot()
}

func (m *moduleContext) InstallBypassMake() bool {
	return m.module.InstallBypassMake()
}

func (m *moduleContext) skipInstall(fullInstallPath InstallPath) bool {
	if m.module.base().commonProperties.SkipInstall {
		return true
	}

	// We'll need a solution for choosing which of modules with the same name in different
	// namespaces to install.  For now, reuse the list of namespaces exported to Make as the
	// list of namespaces to install in a Soong-only build.
	if !m.module.base().commonProperties.NamespaceExportedToMake {
		return true
	}

	if m.Device() {
		if m.Config().EmbeddedInMake() && !m.InstallBypassMake() {
			return true
		}

		if m.Config().SkipMegaDeviceInstall(fullInstallPath.String()) {
			return true
		}
	}

	return false
}

func (m *moduleContext) InstallFile(installPath InstallPath, name string, srcPath Path,
	deps ...Path) InstallPath {
	return m.installFile(installPath, name, srcPath, Cp, deps)
}

func (m *moduleContext) InstallExecutable(installPath InstallPath, name string, srcPath Path,
	deps ...Path) InstallPath {
	return m.installFile(installPath, name, srcPath, CpExecutable, deps)
}

func (m *moduleContext) installFile(installPath InstallPath, name string, srcPath Path,
	rule blueprint.Rule, deps []Path) InstallPath {

	fullInstallPath := installPath.Join(m, name)
	m.module.base().hooks.runInstallHooks(m, fullInstallPath, false)

	if !m.skipInstall(fullInstallPath) {

		deps = append(deps, m.installDeps...)

		var implicitDeps, orderOnlyDeps Paths

		if m.Host() {
			// Installed host modules might be used during the build, depend directly on their
			// dependencies so their timestamp is updated whenever their dependency is updated
			implicitDeps = deps
		} else {
			orderOnlyDeps = deps
		}

		m.Build(pctx, BuildParams{
			Rule:        rule,
			Description: "install " + fullInstallPath.Base(),
			Output:      fullInstallPath,
			Input:       srcPath,
			Implicits:   implicitDeps,
			OrderOnly:   orderOnlyDeps,
			Default:     !m.Config().EmbeddedInMake(),
		})

		m.installFiles = append(m.installFiles, fullInstallPath)
	}
	m.checkbuildFiles = append(m.checkbuildFiles, srcPath)
	return fullInstallPath
}

func (m *moduleContext) InstallSymlink(installPath InstallPath, name string, srcPath InstallPath) InstallPath {
	fullInstallPath := installPath.Join(m, name)
	m.module.base().hooks.runInstallHooks(m, fullInstallPath, true)

	if !m.skipInstall(fullInstallPath) {

		relPath, err := filepath.Rel(path.Dir(fullInstallPath.String()), srcPath.String())
		if err != nil {
			panic(fmt.Sprintf("Unable to generate symlink between %q and %q: %s", fullInstallPath.Base(), srcPath.Base(), err))
		}
		m.Build(pctx, BuildParams{
			Rule:        Symlink,
			Description: "install symlink " + fullInstallPath.Base(),
			Output:      fullInstallPath,
			OrderOnly:   Paths{srcPath},
			Default:     !m.Config().EmbeddedInMake(),
			Args: map[string]string{
				"fromPath": relPath,
			},
		})

		m.installFiles = append(m.installFiles, fullInstallPath)
		m.checkbuildFiles = append(m.checkbuildFiles, srcPath)
	}
	return fullInstallPath
}

// installPath/name -> absPath where absPath might be a path that is available only at runtime
// (e.g. /apex/...)
func (m *moduleContext) InstallAbsoluteSymlink(installPath InstallPath, name string, absPath string) InstallPath {
	fullInstallPath := installPath.Join(m, name)
	m.module.base().hooks.runInstallHooks(m, fullInstallPath, true)

	if !m.skipInstall(fullInstallPath) {
		m.Build(pctx, BuildParams{
			Rule:        Symlink,
			Description: "install symlink " + fullInstallPath.Base() + " -> " + absPath,
			Output:      fullInstallPath,
			Default:     !m.Config().EmbeddedInMake(),
			Args: map[string]string{
				"fromPath": absPath,
			},
		})

		m.installFiles = append(m.installFiles, fullInstallPath)
	}
	return fullInstallPath
}

func (m *moduleContext) CheckbuildFile(srcPath Path) {
	m.checkbuildFiles = append(m.checkbuildFiles, srcPath)
}

type fileInstaller interface {
	filesToInstall() Paths
}

func isFileInstaller(m blueprint.Module) bool {
	_, ok := m.(fileInstaller)
	return ok
}

func isAndroidModule(m blueprint.Module) bool {
	_, ok := m.(Module)
	return ok
}

func findStringInSlice(str string, slice []string) int {
	for i, s := range slice {
		if s == str {
			return i
		}
	}
	return -1
}

// SrcIsModule decodes module references in the format ":name" into the module name, or empty string if the input
// was not a module reference.
func SrcIsModule(s string) (module string) {
	if len(s) > 1 && s[0] == ':' {
		return s[1:]
	}
	return ""
}

// SrcIsModule decodes module references in the format ":name{.tag}" into the module name and tag, ":name" into the
// module name and an empty string for the tag, or empty strings if the input was not a module reference.
func SrcIsModuleWithTag(s string) (module, tag string) {
	if len(s) > 1 && s[0] == ':' {
		module = s[1:]
		if tagStart := strings.IndexByte(module, '{'); tagStart > 0 {
			if module[len(module)-1] == '}' {
				tag = module[tagStart+1 : len(module)-1]
				module = module[:tagStart]
				return module, tag
			}
		}
		return module, ""
	}
	return "", ""
}

type sourceOrOutputDependencyTag struct {
	blueprint.BaseDependencyTag
	tag string
}

func sourceOrOutputDepTag(tag string) blueprint.DependencyTag {
	return sourceOrOutputDependencyTag{tag: tag}
}

var SourceDepTag = sourceOrOutputDepTag("")

// Adds necessary dependencies to satisfy filegroup or generated sources modules listed in srcFiles
// using ":module" syntax, if any.
//
// Deprecated: tag the property with `android:"path"` instead.
func ExtractSourcesDeps(ctx BottomUpMutatorContext, srcFiles []string) {
	set := make(map[string]bool)

	for _, s := range srcFiles {
		if m, t := SrcIsModuleWithTag(s); m != "" {
			if _, found := set[s]; found {
				ctx.ModuleErrorf("found source dependency duplicate: %q!", s)
			} else {
				set[s] = true
				ctx.AddDependency(ctx.Module(), sourceOrOutputDepTag(t), m)
			}
		}
	}
}

// Adds necessary dependencies to satisfy filegroup or generated sources modules specified in s
// using ":module" syntax, if any.
//
// Deprecated: tag the property with `android:"path"` instead.
func ExtractSourceDeps(ctx BottomUpMutatorContext, s *string) {
	if s != nil {
		if m, t := SrcIsModuleWithTag(*s); m != "" {
			ctx.AddDependency(ctx.Module(), sourceOrOutputDepTag(t), m)
		}
	}
}

// A module that implements SourceFileProducer can be referenced from any property that is tagged with `android:"path"`
// using the ":module" syntax and provides a list of paths to be used as if they were listed in the property.
type SourceFileProducer interface {
	Srcs() Paths
}

// A module that implements OutputFileProducer can be referenced from any property that is tagged with `android:"path"`
// using the ":module" syntax or ":module{.tag}" syntax and provides a list of otuput files to be used as if they were
// listed in the property.
type OutputFileProducer interface {
	OutputFiles(tag string) (Paths, error)
}

type HostToolProvider interface {
	HostToolPath() OptionalPath
}

// Returns a list of paths expanded from globs and modules referenced using ":module" syntax.  The property must
// be tagged with `android:"path" to support automatic source module dependency resolution.
//
// Deprecated: use PathsForModuleSrc or PathsForModuleSrcExcludes instead.
func (m *moduleContext) ExpandSources(srcFiles, excludes []string) Paths {
	return PathsForModuleSrcExcludes(m, srcFiles, excludes)
}

// Returns a single path expanded from globs and modules referenced using ":module" syntax.  The property must
// be tagged with `android:"path" to support automatic source module dependency resolution.
//
// Deprecated: use PathForModuleSrc instead.
func (m *moduleContext) ExpandSource(srcFile, prop string) Path {
	return PathForModuleSrc(m, srcFile)
}

// Returns an optional single path expanded from globs and modules referenced using ":module" syntax if
// the srcFile is non-nil.  The property must be tagged with `android:"path" to support automatic source module
// dependency resolution.
func (m *moduleContext) ExpandOptionalSource(srcFile *string, prop string) OptionalPath {
	if srcFile != nil {
		return OptionalPathForPath(PathForModuleSrc(m, *srcFile))
	}
	return OptionalPath{}
}

func (m *moduleContext) RequiredModuleNames() []string {
	return m.module.base().commonProperties.Required
}

func (m *moduleContext) HostRequiredModuleNames() []string {
	return m.module.base().commonProperties.Host_required
}

func (m *moduleContext) TargetRequiredModuleNames() []string {
	return m.module.base().commonProperties.Target_required
}

func (b *baseModuleContext) Glob(globPattern string, excludes []string) Paths {
	ret, err := b.GlobWithDeps(globPattern, excludes)
	if err != nil {
		b.ModuleErrorf("glob: %s", err.Error())
	}
	return pathsForModuleSrcFromFullPath(b, ret, true)
}

func (b *baseModuleContext) GlobFiles(globPattern string, excludes []string) Paths {
	ret, err := b.GlobWithDeps(globPattern, excludes)
	if err != nil {
		b.ModuleErrorf("glob: %s", err.Error())
	}
	return pathsForModuleSrcFromFullPath(b, ret, false)
}

func init() {
	RegisterSingletonType("buildtarget", BuildTargetSingleton)
}

func BuildTargetSingleton() Singleton {
	return &buildTargetSingleton{}
}

func parentDir(dir string) string {
	dir, _ = filepath.Split(dir)
	return filepath.Clean(dir)
}

type buildTargetSingleton struct{}

func (c *buildTargetSingleton) GenerateBuildActions(ctx SingletonContext) {
	var checkbuildDeps Paths

	mmTarget := func(dir string) WritablePath {
		return PathForPhony(ctx,
			"MODULES-IN-"+strings.Replace(filepath.Clean(dir), "/", "-", -1))
	}

	modulesInDir := make(map[string]Paths)

	ctx.VisitAllModules(func(module Module) {
		blueprintDir := module.base().blueprintDir
		installTarget := module.base().installTarget
		checkbuildTarget := module.base().checkbuildTarget

		if checkbuildTarget != nil {
			checkbuildDeps = append(checkbuildDeps, checkbuildTarget)
			modulesInDir[blueprintDir] = append(modulesInDir[blueprintDir], checkbuildTarget)
		}

		if installTarget != nil {
			modulesInDir[blueprintDir] = append(modulesInDir[blueprintDir], installTarget)
		}
	})

	suffix := ""
	if ctx.Config().EmbeddedInMake() {
		suffix = "-soong"
	}

	// Create a top-level checkbuild target that depends on all modules
	ctx.Build(pctx, BuildParams{
		Rule:      blueprint.Phony,
		Output:    PathForPhony(ctx, "checkbuild"+suffix),
		Implicits: checkbuildDeps,
	})

	// Make will generate the MODULES-IN-* targets
	if ctx.Config().EmbeddedInMake() {
		return
	}

	// Ensure ancestor directories are in modulesInDir
	dirs := SortedStringKeys(modulesInDir)
	for _, dir := range dirs {
		dir := parentDir(dir)
		for dir != "." && dir != "/" {
			if _, exists := modulesInDir[dir]; exists {
				break
			}
			modulesInDir[dir] = nil
			dir = parentDir(dir)
		}
	}

	// Make directories build their direct subdirectories
	for _, dir := range dirs {
		p := parentDir(dir)
		if p != "." && p != "/" {
			modulesInDir[p] = append(modulesInDir[p], mmTarget(dir))
		}
	}

	// Create a MODULES-IN-<directory> target that depends on all modules in a directory, and
	// depends on the MODULES-IN-* targets of all of its subdirectories that contain Android.bp
	// files.
	for _, dir := range dirs {
		ctx.Build(pctx, BuildParams{
			Rule:      blueprint.Phony,
			Output:    mmTarget(dir),
			Implicits: modulesInDir[dir],
			// HACK: checkbuild should be an optional build, but force it
			// enabled for now in standalone builds
			Default: !ctx.Config().EmbeddedInMake(),
		})
	}

	// Create (host|host-cross|target)-<OS> phony rules to build a reduced checkbuild.
	osDeps := map[OsType]Paths{}
	ctx.VisitAllModules(func(module Module) {
		if module.Enabled() {
			os := module.Target().Os
			osDeps[os] = append(osDeps[os], module.base().checkbuildFiles...)
		}
	})

	osClass := make(map[string]Paths)
	for os, deps := range osDeps {
		var className string

		switch os.Class {
		case Host:
			className = "host"
		case HostCross:
			className = "host-cross"
		case Device:
			className = "target"
		default:
			continue
		}

		name := PathForPhony(ctx, className+"-"+os.Name)
		osClass[className] = append(osClass[className], name)

		ctx.Build(pctx, BuildParams{
			Rule:      blueprint.Phony,
			Output:    name,
			Implicits: deps,
		})
	}

	// Wrap those into host|host-cross|target phony rules
	for _, class := range SortedStringKeys(osClass) {
		ctx.Build(pctx, BuildParams{
			Rule:      blueprint.Phony,
			Output:    PathForPhony(ctx, class),
			Implicits: osClass[class],
		})
	}
}

// Collect information for opening IDE project files in java/jdeps.go.
type IDEInfo interface {
	IDEInfo(ideInfo *IdeInfo)
	BaseModuleName() string
}

// Extract the base module name from the Import name.
// Often the Import name has a prefix "prebuilt_".
// Remove the prefix explicitly if needed
// until we find a better solution to get the Import name.
type IDECustomizedModuleName interface {
	IDECustomizedModuleName() string
}

type IdeInfo struct {
	Deps              []string `json:"dependencies,omitempty"`
	Srcs              []string `json:"srcs,omitempty"`
	Aidl_include_dirs []string `json:"aidl_include_dirs,omitempty"`
	Jarjar_rules      []string `json:"jarjar_rules,omitempty"`
	Jars              []string `json:"jars,omitempty"`
	Classes           []string `json:"class,omitempty"`
	Installed_paths   []string `json:"installed,omitempty"`
	SrcJars           []string `json:"srcjars,omitempty"`
}
