// Copyright 2016 Google Inc. All rights reserved.
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

package cc

import (
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/google/blueprint/pathtools"

	"android/soong/android"
	"android/soong/cc/config"
	"android/soong/genrule"
)

type LibraryProperties struct {
	// local file name to pass to the linker as -unexported_symbols_list
	Unexported_symbols_list *string `android:"path,arch_variant"`
	// local file name to pass to the linker as -force_symbols_not_weak_list
	Force_symbols_not_weak_list *string `android:"path,arch_variant"`
	// local file name to pass to the linker as -force_symbols_weak_list
	Force_symbols_weak_list *string `android:"path,arch_variant"`

	// rename host libraries to prevent overlap with system installed libraries
	Unique_host_soname *bool

	Aidl struct {
		// export headers generated from .aidl sources
		Export_aidl_headers *bool
	}

	Proto struct {
		// export headers generated from .proto sources
		Export_proto_headers *bool
	}

	Sysprop struct {
		// Whether platform owns this sysprop library.
		Platform *bool
	} `blueprint:"mutated"`

	Static_ndk_lib *bool

	Stubs struct {
		// Relative path to the symbol map. The symbol map provides the list of
		// symbols that are exported for stubs variant of this library.
		Symbol_file *string `android:"path"`

		// List versions to generate stubs libs for.
		Versions []string
	}

	// set the name of the output
	Stem *string `android:"arch_variant"`

	// set suffix of the name of the output
	Suffix *string `android:"arch_variant"`

	Target struct {
		Vendor struct {
			// set suffix of the name of the output
			Suffix *string `android:"arch_variant"`
		}
	}

	// Names of modules to be overridden. Listed modules can only be other shared libraries
	// (in Make or Soong).
	// This does not completely prevent installation of the overridden libraries, but if both
	// binaries would be installed by default (in PRODUCT_PACKAGES) the other library will be removed
	// from PRODUCT_PACKAGES.
	Overrides []string

	// Properties for ABI compatibility checker
	Header_abi_checker struct {
		// Enable ABI checks (even if this is not an LLNDK/VNDK lib)
		Enabled *bool

		// Path to a symbol file that specifies the symbols to be included in the generated
		// ABI dump file
		Symbol_file *string `android:"path"`

		// Symbol versions that should be ignored from the symbol file
		Exclude_symbol_versions []string

		// Symbol tags that should be ignored from the symbol file
		Exclude_symbol_tags []string
	}

	// Order symbols in .bss section by their sizes.  Only useful for shared libraries.
	Sort_bss_symbols_by_size *bool

	// Inject boringssl hash into the shared library.  This is only intended for use by external/boringssl.
	Inject_bssl_hash *bool `android:"arch_variant"`
}

type StaticProperties struct {
	Static StaticOrSharedProperties `android:"arch_variant"`
}

type SharedProperties struct {
	Shared StaticOrSharedProperties `android:"arch_variant"`
}

type StaticOrSharedProperties struct {
	Srcs   []string `android:"path,arch_variant"`
	Cflags []string `android:"arch_variant"`

	Enabled            *bool    `android:"arch_variant"`
	Whole_static_libs  []string `android:"arch_variant"`
	Static_libs        []string `android:"arch_variant"`
	Shared_libs        []string `android:"arch_variant"`
	System_shared_libs []string `android:"arch_variant"`

	Export_shared_lib_headers []string `android:"arch_variant"`
	Export_static_lib_headers []string `android:"arch_variant"`

	Apex_available []string `android:"arch_variant"`
}

type LibraryMutatedProperties struct {
	// Build a static variant
	BuildStatic bool `blueprint:"mutated"`
	// Build a shared variant
	BuildShared bool `blueprint:"mutated"`
	// This variant is shared
	VariantIsShared bool `blueprint:"mutated"`
	// This variant is static
	VariantIsStatic bool `blueprint:"mutated"`

	// This variant is a stubs lib
	BuildStubs bool `blueprint:"mutated"`
	// Version of the stubs lib
	StubsVersion string `blueprint:"mutated"`
}

type FlagExporterProperties struct {
	// list of directories relative to the Blueprints file that will
	// be added to the include path (using -I) for this module and any module that links
	// against this module.  Directories listed in export_include_dirs do not need to be
	// listed in local_include_dirs.
	Export_include_dirs []string `android:"arch_variant"`

	Target struct {
		Vendor struct {
			// list of exported include directories, like
			// export_include_dirs, that will be applied to the
			// vendor variant of this library. This will overwrite
			// any other declarations.
			Override_export_include_dirs []string
		}
	}
}

func init() {
	android.RegisterModuleType("cc_library_static", LibraryStaticFactory)
	android.RegisterModuleType("cc_library_shared", LibrarySharedFactory)
	android.RegisterModuleType("cc_library", LibraryFactory)
	android.RegisterModuleType("cc_library_host_static", LibraryHostStaticFactory)
	android.RegisterModuleType("cc_library_host_shared", LibraryHostSharedFactory)
	android.RegisterModuleType("cc_library_headers", LibraryHeaderFactory)
}

// cc_library creates both static and/or shared libraries for a device and/or
// host. By default, a cc_library has a single variant that targets the device.
// Specifying `host_supported: true` also creates a library that targets the
// host.
func LibraryFactory() android.Module {
	module, _ := NewLibrary(android.HostAndDeviceSupported)
	return module.Init()
}

// cc_library_static creates a static library for a device and/or host binary.
func LibraryStaticFactory() android.Module {
	module, library := NewLibrary(android.HostAndDeviceSupported)
	library.BuildOnlyStatic()
	return module.Init()
}

// cc_library_shared creates a shared library for a device and/or host.
func LibrarySharedFactory() android.Module {
	module, library := NewLibrary(android.HostAndDeviceSupported)
	library.BuildOnlyShared()
	return module.Init()
}

// cc_library_host_static creates a static library that is linkable to a host
// binary.
func LibraryHostStaticFactory() android.Module {
	module, library := NewLibrary(android.HostSupported)
	library.BuildOnlyStatic()
	return module.Init()
}

// cc_library_host_shared creates a shared library that is usable on a host.
func LibraryHostSharedFactory() android.Module {
	module, library := NewLibrary(android.HostSupported)
	library.BuildOnlyShared()
	return module.Init()
}

// cc_library_headers contains a set of c/c++ headers which are imported by
// other soong cc modules using the header_libs property. For best practices,
// use export_include_dirs property or LOCAL_EXPORT_C_INCLUDE_DIRS for
// Make.
func LibraryHeaderFactory() android.Module {
	module, library := NewLibrary(android.HostAndDeviceSupported)
	library.HeaderOnly()
	return module.Init()
}

type flagExporter struct {
	Properties FlagExporterProperties

	dirs       []string
	systemDirs []string
	flags      []string
	deps       android.Paths
}

func (f *flagExporter) exportedIncludes(ctx ModuleContext) android.Paths {
	if ctx.useVndk() && f.Properties.Target.Vendor.Override_export_include_dirs != nil {
		return android.PathsForModuleSrc(ctx, f.Properties.Target.Vendor.Override_export_include_dirs)
	} else {
		return android.PathsForModuleSrc(ctx, f.Properties.Export_include_dirs)
	}
}

func (f *flagExporter) exportIncludes(ctx ModuleContext) {
	f.dirs = append(f.dirs, f.exportedIncludes(ctx).Strings()...)
}

func (f *flagExporter) exportIncludesAsSystem(ctx ModuleContext) {
	f.systemDirs = append(f.systemDirs, f.exportedIncludes(ctx).Strings()...)
}

func (f *flagExporter) reexportDirs(dirs ...string) {
	f.dirs = append(f.dirs, dirs...)
}

func (f *flagExporter) reexportSystemDirs(dirs ...string) {
	f.systemDirs = append(f.systemDirs, dirs...)
}

func (f *flagExporter) reexportFlags(flags ...string) {
	for _, flag := range flags {
		if strings.HasPrefix(flag, "-I") || strings.HasPrefix(flag, "-isystem") {
			panic(fmt.Errorf("Exporting invalid flag %q: "+
				"use reexportDirs or reexportSystemDirs to export directories", flag))
		}
	}
	f.flags = append(f.flags, flags...)
}

func (f *flagExporter) reexportDeps(deps ...android.Path) {
	f.deps = append(f.deps, deps...)
}

func (f *flagExporter) exportedDirs() []string {
	return f.dirs
}

func (f *flagExporter) exportedSystemDirs() []string {
	return f.systemDirs
}

func (f *flagExporter) exportedFlags() []string {
	return f.flags
}

func (f *flagExporter) exportedDeps() android.Paths {
	return f.deps
}

type exportedFlagsProducer interface {
	exportedDirs() []string
	exportedSystemDirs() []string
	exportedFlags() []string
	exportedDeps() android.Paths
}

var _ exportedFlagsProducer = (*flagExporter)(nil)

// libraryDecorator wraps baseCompiler, baseLinker and baseInstaller to provide library-specific
// functionality: static vs. shared linkage, reusing object files for shared libraries
type libraryDecorator struct {
	Properties        LibraryProperties
	StaticProperties  StaticProperties
	SharedProperties  SharedProperties
	MutatedProperties LibraryMutatedProperties

	// For reusing static library objects for shared library
	reuseObjects Objects

	// table-of-contents file to optimize out relinking when possible
	tocFile android.OptionalPath

	flagExporter
	stripper

	// If we're used as a whole_static_lib, our missing dependencies need
	// to be given
	wholeStaticMissingDeps []string

	// For whole_static_libs
	objects Objects

	// Uses the module's name if empty, but can be overridden. Does not include
	// shlib suffix.
	libName string

	sabi *sabi

	// Output archive of gcno coverage information files
	coverageOutputFile android.OptionalPath

	// linked Source Abi Dump
	sAbiOutputFile android.OptionalPath

	// Source Abi Diff
	sAbiDiff android.OptionalPath

	// Location of the static library in the sysroot. Empty if the library is
	// not included in the NDK.
	ndkSysrootPath android.Path

	// Location of the linked, unstripped library for shared libraries
	unstrippedOutputFile android.Path

	// Location of the file that should be copied to dist dir when requested
	distFile android.OptionalPath

	versionScriptPath android.ModuleGenPath

	post_install_cmds []string

	// If useCoreVariant is true, the vendor variant of a VNDK library is
	// not installed.
	useCoreVariant bool

	// Decorated interafaces
	*baseCompiler
	*baseLinker
	*baseInstaller
}

func (library *libraryDecorator) linkerProps() []interface{} {
	var props []interface{}
	props = append(props, library.baseLinker.linkerProps()...)
	props = append(props,
		&library.Properties,
		&library.MutatedProperties,
		&library.flagExporter.Properties,
		&library.stripper.StripProperties)

	if library.MutatedProperties.BuildShared {
		props = append(props, &library.SharedProperties)
	}
	if library.MutatedProperties.BuildStatic {
		props = append(props, &library.StaticProperties)
	}

	return props
}

func (library *libraryDecorator) linkerFlags(ctx ModuleContext, flags Flags) Flags {
	flags = library.baseLinker.linkerFlags(ctx, flags)

	// MinGW spits out warnings about -fPIC even for -fpie?!) being ignored because
	// all code is position independent, and then those warnings get promoted to
	// errors.
	if !ctx.Windows() {
		flags.CFlags = append(flags.CFlags, "-fPIC")
	}

	if library.static() {
		flags.CFlags = append(flags.CFlags, library.StaticProperties.Static.Cflags...)
	} else if library.shared() {
		flags.CFlags = append(flags.CFlags, library.SharedProperties.Shared.Cflags...)
	}

	if library.shared() {
		libName := library.getLibName(ctx)
		var f []string
		if ctx.toolchain().Bionic() {
			f = append(f,
				"-nostdlib",
				"-Wl,--gc-sections",
			)
		}

		if ctx.Darwin() {
			f = append(f,
				"-dynamiclib",
				"-single_module",
				"-install_name @rpath/"+libName+flags.Toolchain.ShlibSuffix(),
			)
			if ctx.Arch().ArchType == android.X86 {
				f = append(f,
					"-read_only_relocs suppress",
				)
			}
		} else {
			f = append(f, "-shared")
			if !ctx.Windows() {
				f = append(f, "-Wl,-soname,"+libName+flags.Toolchain.ShlibSuffix())
			}
		}

		flags.LdFlags = append(f, flags.LdFlags...)
	}

	return flags
}

func (library *libraryDecorator) compilerFlags(ctx ModuleContext, flags Flags, deps PathDeps) Flags {
	exportIncludeDirs := library.flagExporter.exportedIncludes(ctx)
	if len(exportIncludeDirs) > 0 {
		f := includeDirsToFlags(exportIncludeDirs)
		flags.GlobalFlags = append(flags.GlobalFlags, f)
		flags.YasmFlags = append(flags.YasmFlags, f)
	}

	flags = library.baseCompiler.compilerFlags(ctx, flags, deps)
	if library.buildStubs() {
		// Remove -include <file> when compiling stubs. Otherwise, the force included
		// headers might cause conflicting types error with the symbols in the
		// generated stubs source code. e.g.
		// double acos(double); // in header
		// void acos() {} // in the generated source code
		removeInclude := func(flags []string) []string {
			ret := flags[:0]
			for _, f := range flags {
				if strings.HasPrefix(f, "-include ") {
					continue
				}
				ret = append(ret, f)
			}
			return ret
		}
		flags.GlobalFlags = removeInclude(flags.GlobalFlags)
		flags.CFlags = removeInclude(flags.CFlags)

		flags = addStubLibraryCompilerFlags(flags)
	}
	return flags
}

// Returns a string that represents the class of the ABI dump.
// Returns an empty string if ABI check is disabled for this library.
func (library *libraryDecorator) classifySourceAbiDump(ctx ModuleContext) string {
	enabled := library.Properties.Header_abi_checker.Enabled
	if enabled != nil && !Bool(enabled) {
		return ""
	}
	// Return NDK if the library is both NDK and LLNDK.
	if ctx.isNdk() {
		return "NDK"
	}
	if ctx.isLlndkPublic(ctx.Config()) {
		return "LLNDK"
	}
	if ctx.useVndk() && ctx.isVndk() && !ctx.isVndkPrivate(ctx.Config()) {
		if ctx.isVndkSp() {
			if ctx.isVndkExt() {
				return "VNDK-SP-ext"
			} else {
				return "VNDK-SP"
			}
		} else {
			if ctx.isVndkExt() {
				return "VNDK-ext"
			} else {
				return "VNDK-core"
			}
		}
	}
	if Bool(enabled) || ctx.hasStubsVariants() {
		return "PLATFORM"
	}
	return ""
}

func (library *libraryDecorator) shouldCreateSourceAbiDump(ctx ModuleContext) bool {
	if !ctx.shouldCreateSourceAbiDump() {
		return false
	}
	return library.classifySourceAbiDump(ctx) != ""
}

func (library *libraryDecorator) compile(ctx ModuleContext, flags Flags, deps PathDeps) Objects {
	if library.buildStubs() {
		objs, versionScript := compileStubLibrary(ctx, flags, String(library.Properties.Stubs.Symbol_file), library.MutatedProperties.StubsVersion, "--apex")
		library.versionScriptPath = versionScript
		return objs
	}

	if !library.buildShared() && !library.buildStatic() {
		if len(library.baseCompiler.Properties.Srcs) > 0 {
			ctx.PropertyErrorf("srcs", "cc_library_headers must not have any srcs")
		}
		if len(library.StaticProperties.Static.Srcs) > 0 {
			ctx.PropertyErrorf("static.srcs", "cc_library_headers must not have any srcs")
		}
		if len(library.SharedProperties.Shared.Srcs) > 0 {
			ctx.PropertyErrorf("shared.srcs", "cc_library_headers must not have any srcs")
		}
		return Objects{}
	}
	if library.shouldCreateSourceAbiDump(ctx) || library.sabi.Properties.CreateSAbiDumps {
		exportIncludeDirs := library.flagExporter.exportedIncludes(ctx)
		var SourceAbiFlags []string
		for _, dir := range exportIncludeDirs.Strings() {
			SourceAbiFlags = append(SourceAbiFlags, "-I"+dir)
		}
		for _, reexportedInclude := range library.sabi.Properties.ReexportedIncludes {
			SourceAbiFlags = append(SourceAbiFlags, "-I"+reexportedInclude)
		}
		flags.SAbiFlags = SourceAbiFlags
		total_length := len(library.baseCompiler.Properties.Srcs) + len(deps.GeneratedSources) +
			len(library.SharedProperties.Shared.Srcs) + len(library.StaticProperties.Static.Srcs)
		if total_length > 0 {
			flags.SAbiDump = true
		}
	}
	objs := library.baseCompiler.compile(ctx, flags, deps)
	library.reuseObjects = objs
	buildFlags := flagsToBuilderFlags(flags)

	if library.static() {
		srcs := android.PathsForModuleSrc(ctx, library.StaticProperties.Static.Srcs)
		objs = objs.Append(compileObjs(ctx, buildFlags, android.DeviceStaticLibrary,
			srcs, library.baseCompiler.pathDeps, library.baseCompiler.cFlagsDeps))
	} else if library.shared() {
		srcs := android.PathsForModuleSrc(ctx, library.SharedProperties.Shared.Srcs)
		objs = objs.Append(compileObjs(ctx, buildFlags, android.DeviceSharedLibrary,
			srcs, library.baseCompiler.pathDeps, library.baseCompiler.cFlagsDeps))
	}

	return objs
}

type libraryInterface interface {
	getWholeStaticMissingDeps() []string
	static() bool
	shared() bool
	objs() Objects
	reuseObjs() (Objects, exportedFlagsProducer)
	toc() android.OptionalPath

	// Returns true if the build options for the module have selected a static or shared build
	buildStatic() bool
	buildShared() bool

	// Sets whether a specific variant is static or shared
	setStatic()
	setShared()

	// Write LOCAL_ADDITIONAL_DEPENDENCIES for ABI diff
	androidMkWriteAdditionalDependenciesForSourceAbiDiff(w io.Writer)

	availableFor(string) bool
}

func (library *libraryDecorator) getLibName(ctx BaseModuleContext) string {
	name := library.libName
	if name == "" {
		name = String(library.Properties.Stem)
		if name == "" {
			name = ctx.baseModuleName()
		}
	}

	suffix := ""
	if ctx.useVndk() {
		suffix = String(library.Properties.Target.Vendor.Suffix)
	}
	if suffix == "" {
		suffix = String(library.Properties.Suffix)
	}

	name += suffix

	if ctx.isVndkExt() {
		// vndk-ext lib should have the same name with original lib
		ctx.VisitDirectDepsWithTag(vndkExtDepTag, func(module android.Module) {
			originalName := module.(*Module).outputFile.Path()
			name = strings.TrimSuffix(originalName.Base(), originalName.Ext())
		})
	}

	if ctx.Host() && Bool(library.Properties.Unique_host_soname) {
		if !strings.HasSuffix(name, "-host") {
			name = name + "-host"
		}
	}

	return name
}

var versioningMacroNamesListMutex sync.Mutex

func (library *libraryDecorator) linkerInit(ctx BaseModuleContext) {
	location := InstallInSystem
	if library.baseLinker.sanitize.inSanitizerDir() {
		location = InstallInSanitizerDir
	}
	library.baseInstaller.location = location
	library.baseLinker.linkerInit(ctx)
	// Let baseLinker know whether this variant is for stubs or not, so that
	// it can omit things that are not required for linking stubs.
	library.baseLinker.dynamicProperties.BuildStubs = library.buildStubs()

	if library.buildStubs() {
		macroNames := versioningMacroNamesList(ctx.Config())
		myName := versioningMacroName(ctx.ModuleName())
		versioningMacroNamesListMutex.Lock()
		defer versioningMacroNamesListMutex.Unlock()
		if (*macroNames)[myName] == "" {
			(*macroNames)[myName] = ctx.ModuleName()
		} else if (*macroNames)[myName] != ctx.ModuleName() {
			ctx.ModuleErrorf("Macro name %q for versioning conflicts with macro name from module %q ", myName, (*macroNames)[myName])
		}
	}
}

func (library *libraryDecorator) compilerDeps(ctx DepsContext, deps Deps) Deps {
	deps = library.baseCompiler.compilerDeps(ctx, deps)

	return deps
}

func (library *libraryDecorator) linkerDeps(ctx DepsContext, deps Deps) Deps {
	if library.static() {
		if library.StaticProperties.Static.System_shared_libs != nil {
			library.baseLinker.Properties.System_shared_libs = library.StaticProperties.Static.System_shared_libs
		}
	} else if library.shared() {
		if library.SharedProperties.Shared.System_shared_libs != nil {
			library.baseLinker.Properties.System_shared_libs = library.SharedProperties.Shared.System_shared_libs
		}
	}

	deps = library.baseLinker.linkerDeps(ctx, deps)

	if library.static() {
		deps.WholeStaticLibs = append(deps.WholeStaticLibs,
			library.StaticProperties.Static.Whole_static_libs...)
		deps.StaticLibs = append(deps.StaticLibs, library.StaticProperties.Static.Static_libs...)
		deps.SharedLibs = append(deps.SharedLibs, library.StaticProperties.Static.Shared_libs...)

		deps.ReexportSharedLibHeaders = append(deps.ReexportSharedLibHeaders, library.StaticProperties.Static.Export_shared_lib_headers...)
		deps.ReexportStaticLibHeaders = append(deps.ReexportStaticLibHeaders, library.StaticProperties.Static.Export_static_lib_headers...)
	} else if library.shared() {
		if ctx.toolchain().Bionic() && !Bool(library.baseLinker.Properties.Nocrt) {
			if !ctx.useSdk() {
				deps.CrtBegin = "crtbegin_so"
				deps.CrtEnd = "crtend_so"
			} else {
				// TODO(danalbert): Add generation of crt objects.
				// For `sdk_version: "current"`, we don't actually have a
				// freshly generated set of CRT objects. Use the last stable
				// version.
				version := ctx.sdkVersion()
				if version == "current" {
					version = getCurrentNdkPrebuiltVersion(ctx)
				}
				deps.CrtBegin = "ndk_crtbegin_so." + version
				deps.CrtEnd = "ndk_crtend_so." + version
			}
		}
		deps.WholeStaticLibs = append(deps.WholeStaticLibs, library.SharedProperties.Shared.Whole_static_libs...)
		deps.StaticLibs = append(deps.StaticLibs, library.SharedProperties.Shared.Static_libs...)
		deps.SharedLibs = append(deps.SharedLibs, library.SharedProperties.Shared.Shared_libs...)

		deps.ReexportSharedLibHeaders = append(deps.ReexportSharedLibHeaders, library.SharedProperties.Shared.Export_shared_lib_headers...)
		deps.ReexportStaticLibHeaders = append(deps.ReexportStaticLibHeaders, library.SharedProperties.Shared.Export_static_lib_headers...)
	}
	if ctx.useVndk() {
		deps.WholeStaticLibs = removeListFromList(deps.WholeStaticLibs, library.baseLinker.Properties.Target.Vendor.Exclude_static_libs)
		deps.SharedLibs = removeListFromList(deps.SharedLibs, library.baseLinker.Properties.Target.Vendor.Exclude_shared_libs)
		deps.StaticLibs = removeListFromList(deps.StaticLibs, library.baseLinker.Properties.Target.Vendor.Exclude_static_libs)
		deps.ReexportSharedLibHeaders = removeListFromList(deps.ReexportSharedLibHeaders, library.baseLinker.Properties.Target.Vendor.Exclude_shared_libs)
		deps.ReexportStaticLibHeaders = removeListFromList(deps.ReexportStaticLibHeaders, library.baseLinker.Properties.Target.Vendor.Exclude_static_libs)
	}
	if ctx.inRecovery() {
		deps.WholeStaticLibs = removeListFromList(deps.WholeStaticLibs, library.baseLinker.Properties.Target.Recovery.Exclude_static_libs)
		deps.SharedLibs = removeListFromList(deps.SharedLibs, library.baseLinker.Properties.Target.Recovery.Exclude_shared_libs)
		deps.StaticLibs = removeListFromList(deps.StaticLibs, library.baseLinker.Properties.Target.Recovery.Exclude_static_libs)
		deps.ReexportSharedLibHeaders = removeListFromList(deps.ReexportSharedLibHeaders, library.baseLinker.Properties.Target.Recovery.Exclude_shared_libs)
		deps.ReexportStaticLibHeaders = removeListFromList(deps.ReexportStaticLibHeaders, library.baseLinker.Properties.Target.Recovery.Exclude_static_libs)
	}

	return deps
}

func (library *libraryDecorator) linkStatic(ctx ModuleContext,
	flags Flags, deps PathDeps, objs Objects) android.Path {

	library.objects = deps.WholeStaticLibObjs.Copy()
	library.objects = library.objects.Append(objs)

	fileName := ctx.ModuleName() + staticLibraryExtension
	outputFile := android.PathForModuleOut(ctx, fileName)
	builderFlags := flagsToBuilderFlags(flags)

	if Bool(library.baseLinker.Properties.Use_version_lib) {
		if ctx.Host() {
			versionedOutputFile := outputFile
			outputFile = android.PathForModuleOut(ctx, "unversioned", fileName)
			library.injectVersionSymbol(ctx, outputFile, versionedOutputFile)
		} else {
			versionedOutputFile := android.PathForModuleOut(ctx, "versioned", fileName)
			library.distFile = android.OptionalPathForPath(versionedOutputFile)
			library.injectVersionSymbol(ctx, outputFile, versionedOutputFile)
		}
	}

	TransformObjToStaticLib(ctx, library.objects.objFiles, builderFlags, outputFile, objs.tidyFiles)

	library.coverageOutputFile = TransformCoverageFilesToZip(ctx, library.objects, ctx.ModuleName())

	library.wholeStaticMissingDeps = ctx.GetMissingDependencies()

	ctx.CheckbuildFile(outputFile)

	return outputFile
}

func (library *libraryDecorator) linkShared(ctx ModuleContext,
	flags Flags, deps PathDeps, objs Objects) android.Path {

	var linkerDeps android.Paths
	linkerDeps = append(linkerDeps, flags.LdFlagsDeps...)

	unexportedSymbols := ctx.ExpandOptionalSource(library.Properties.Unexported_symbols_list, "unexported_symbols_list")
	forceNotWeakSymbols := ctx.ExpandOptionalSource(library.Properties.Force_symbols_not_weak_list, "force_symbols_not_weak_list")
	forceWeakSymbols := ctx.ExpandOptionalSource(library.Properties.Force_symbols_weak_list, "force_symbols_weak_list")
	if !ctx.Darwin() {
		if unexportedSymbols.Valid() {
			ctx.PropertyErrorf("unexported_symbols_list", "Only supported on Darwin")
		}
		if forceNotWeakSymbols.Valid() {
			ctx.PropertyErrorf("force_symbols_not_weak_list", "Only supported on Darwin")
		}
		if forceWeakSymbols.Valid() {
			ctx.PropertyErrorf("force_symbols_weak_list", "Only supported on Darwin")
		}
	} else {
		if unexportedSymbols.Valid() {
			flags.LdFlags = append(flags.LdFlags, "-Wl,-unexported_symbols_list,"+unexportedSymbols.String())
			linkerDeps = append(linkerDeps, unexportedSymbols.Path())
		}
		if forceNotWeakSymbols.Valid() {
			flags.LdFlags = append(flags.LdFlags, "-Wl,-force_symbols_not_weak_list,"+forceNotWeakSymbols.String())
			linkerDeps = append(linkerDeps, forceNotWeakSymbols.Path())
		}
		if forceWeakSymbols.Valid() {
			flags.LdFlags = append(flags.LdFlags, "-Wl,-force_symbols_weak_list,"+forceWeakSymbols.String())
			linkerDeps = append(linkerDeps, forceWeakSymbols.Path())
		}
	}
	if library.buildStubs() {
		linkerScriptFlags := "-Wl,--version-script," + library.versionScriptPath.String()
		flags.LdFlags = append(flags.LdFlags, linkerScriptFlags)
		linkerDeps = append(linkerDeps, library.versionScriptPath)
	}

	fileName := library.getLibName(ctx) + flags.Toolchain.ShlibSuffix()
	outputFile := android.PathForModuleOut(ctx, fileName)
	ret := outputFile

	var implicitOutputs android.WritablePaths
	if ctx.Windows() {
		importLibraryPath := android.PathForModuleOut(ctx, pathtools.ReplaceExtension(fileName, "lib"))

		flags.LdFlags = append(flags.LdFlags, "-Wl,--out-implib="+importLibraryPath.String())
		implicitOutputs = append(implicitOutputs, importLibraryPath)
	}

	builderFlags := flagsToBuilderFlags(flags)

	// Optimize out relinking against shared libraries whose interface hasn't changed by
	// depending on a table of contents file instead of the library itself.
	tocFile := outputFile.ReplaceExtension(ctx, flags.Toolchain.ShlibSuffix()[1:]+".toc")
	library.tocFile = android.OptionalPathForPath(tocFile)
	TransformSharedObjectToToc(ctx, outputFile, tocFile, builderFlags)

	if library.stripper.needsStrip(ctx) {
		if ctx.Darwin() {
			builderFlags.stripUseGnuStrip = true
		}
		strippedOutputFile := outputFile
		outputFile = android.PathForModuleOut(ctx, "unstripped", fileName)
		library.stripper.stripExecutableOrSharedLib(ctx, outputFile, strippedOutputFile, builderFlags)
	}
	library.unstrippedOutputFile = outputFile

	outputFile = maybeInjectBoringSSLHash(ctx, outputFile, library.Properties.Inject_bssl_hash, fileName)

	if Bool(library.baseLinker.Properties.Use_version_lib) {
		if ctx.Host() {
			versionedOutputFile := outputFile
			outputFile = android.PathForModuleOut(ctx, "unversioned", fileName)
			library.injectVersionSymbol(ctx, outputFile, versionedOutputFile)
		} else {
			versionedOutputFile := android.PathForModuleOut(ctx, "versioned", fileName)
			library.distFile = android.OptionalPathForPath(versionedOutputFile)

			if library.stripper.needsStrip(ctx) {
				out := android.PathForModuleOut(ctx, "versioned-stripped", fileName)
				library.distFile = android.OptionalPathForPath(out)
				library.stripper.stripExecutableOrSharedLib(ctx, versionedOutputFile, out, builderFlags)
			}

			library.injectVersionSymbol(ctx, outputFile, versionedOutputFile)
		}
	}

	sharedLibs := deps.EarlySharedLibs
	sharedLibs = append(sharedLibs, deps.SharedLibs...)
	sharedLibs = append(sharedLibs, deps.LateSharedLibs...)

	linkerDeps = append(linkerDeps, deps.EarlySharedLibsDeps...)
	linkerDeps = append(linkerDeps, deps.SharedLibsDeps...)
	linkerDeps = append(linkerDeps, deps.LateSharedLibsDeps...)
	linkerDeps = append(linkerDeps, objs.tidyFiles...)

	if Bool(library.Properties.Sort_bss_symbols_by_size) {
		unsortedOutputFile := android.PathForModuleOut(ctx, "unsorted", fileName)
		TransformObjToDynamicBinary(ctx, objs.objFiles, sharedLibs,
			deps.StaticLibs, deps.LateStaticLibs, deps.WholeStaticLibs,
			linkerDeps, deps.CrtBegin, deps.CrtEnd, false, builderFlags, unsortedOutputFile, implicitOutputs)

		symbolOrderingFile := android.PathForModuleOut(ctx, "unsorted", fileName+".symbol_order")
		symbolOrderingFlag := library.baseLinker.sortBssSymbolsBySize(ctx, unsortedOutputFile, symbolOrderingFile, builderFlags)
		builderFlags.ldFlags += " " + symbolOrderingFlag
		linkerDeps = append(linkerDeps, symbolOrderingFile)
	}

	TransformObjToDynamicBinary(ctx, objs.objFiles, sharedLibs,
		deps.StaticLibs, deps.LateStaticLibs, deps.WholeStaticLibs,
		linkerDeps, deps.CrtBegin, deps.CrtEnd, false, builderFlags, outputFile, implicitOutputs)

	objs.coverageFiles = append(objs.coverageFiles, deps.StaticLibObjs.coverageFiles...)
	objs.coverageFiles = append(objs.coverageFiles, deps.WholeStaticLibObjs.coverageFiles...)

	objs.sAbiDumpFiles = append(objs.sAbiDumpFiles, deps.StaticLibObjs.sAbiDumpFiles...)
	objs.sAbiDumpFiles = append(objs.sAbiDumpFiles, deps.WholeStaticLibObjs.sAbiDumpFiles...)

	library.coverageOutputFile = TransformCoverageFilesToZip(ctx, objs, library.getLibName(ctx))
	library.linkSAbiDumpFiles(ctx, objs, fileName, ret)

	return ret
}

func (library *libraryDecorator) unstrippedOutputFilePath() android.Path {
	return library.unstrippedOutputFile
}

func (library *libraryDecorator) nativeCoverage() bool {
	if library.header() || library.buildStubs() {
		return false
	}
	return true
}

func (library *libraryDecorator) coverageOutputFilePath() android.OptionalPath {
	return library.coverageOutputFile
}

func getRefAbiDumpFile(ctx ModuleContext, vndkVersion, fileName string) android.Path {
	isNdk := ctx.isNdk()
	isLlndkOrVndk := ctx.isLlndkPublic(ctx.Config()) || ctx.isVndk()

	refAbiDumpTextFile := android.PathForVndkRefAbiDump(ctx, vndkVersion, fileName, isNdk, isLlndkOrVndk, false)
	refAbiDumpGzipFile := android.PathForVndkRefAbiDump(ctx, vndkVersion, fileName, isNdk, isLlndkOrVndk, true)

	if refAbiDumpTextFile.Valid() {
		if refAbiDumpGzipFile.Valid() {
			ctx.ModuleErrorf(
				"Two reference ABI dump files are found: %q and %q. Please delete the stale one.",
				refAbiDumpTextFile, refAbiDumpGzipFile)
			return nil
		}
		return refAbiDumpTextFile.Path()
	}
	if refAbiDumpGzipFile.Valid() {
		return UnzipRefDump(ctx, refAbiDumpGzipFile.Path(), fileName)
	}
	return nil
}

func (library *libraryDecorator) linkSAbiDumpFiles(ctx ModuleContext, objs Objects, fileName string, soFile android.Path) {
	if library.shouldCreateSourceAbiDump(ctx) {
		vndkVersion := ctx.DeviceConfig().PlatformVndkVersion()
		if ver := ctx.DeviceConfig().VndkVersion(); ver != "" && ver != "current" {
			vndkVersion = ver
		}

		exportIncludeDirs := library.flagExporter.exportedIncludes(ctx)
		var SourceAbiFlags []string
		for _, dir := range exportIncludeDirs.Strings() {
			SourceAbiFlags = append(SourceAbiFlags, "-I"+dir)
		}
		for _, reexportedInclude := range library.sabi.Properties.ReexportedIncludes {
			SourceAbiFlags = append(SourceAbiFlags, "-I"+reexportedInclude)
		}
		exportedHeaderFlags := strings.Join(SourceAbiFlags, " ")
		library.sAbiOutputFile = TransformDumpToLinkedDump(ctx, objs.sAbiDumpFiles, soFile, fileName, exportedHeaderFlags,
			android.OptionalPathForModuleSrc(ctx, library.symbolFileForAbiCheck(ctx)),
			library.Properties.Header_abi_checker.Exclude_symbol_versions,
			library.Properties.Header_abi_checker.Exclude_symbol_tags)

		addLsdumpPath(library.classifySourceAbiDump(ctx) + ":" + library.sAbiOutputFile.String())

		refAbiDumpFile := getRefAbiDumpFile(ctx, vndkVersion, fileName)
		if refAbiDumpFile != nil {
			library.sAbiDiff = SourceAbiDiff(ctx, library.sAbiOutputFile.Path(),
				refAbiDumpFile, fileName, exportedHeaderFlags, ctx.isLlndk(ctx.Config()), ctx.isNdk(), ctx.isVndkExt())
		}
	}
}

func (library *libraryDecorator) link(ctx ModuleContext,
	flags Flags, deps PathDeps, objs Objects) android.Path {

	objs = deps.Objs.Copy().Append(objs)
	var out android.Path
	if library.static() || library.header() {
		out = library.linkStatic(ctx, flags, deps, objs)
	} else {
		out = library.linkShared(ctx, flags, deps, objs)
	}

	library.exportIncludes(ctx)
	library.reexportDirs(deps.ReexportedDirs...)
	library.reexportSystemDirs(deps.ReexportedSystemDirs...)
	library.reexportFlags(deps.ReexportedFlags...)
	library.reexportDeps(deps.ReexportedDeps...)

	if Bool(library.Properties.Aidl.Export_aidl_headers) {
		if library.baseCompiler.hasSrcExt(".aidl") {
			dir := android.PathForModuleGen(ctx, "aidl").String()
			library.reexportDirs(dir)
			library.reexportDeps(library.baseCompiler.pathDeps...) // TODO: restrict to aidl deps
		}
	}

	if Bool(library.Properties.Proto.Export_proto_headers) {
		if library.baseCompiler.hasSrcExt(".proto") {
			includes := []string{}
			if flags.proto.CanonicalPathFromRoot {
				includes = append(includes, flags.proto.SubDir.String())
			}
			includes = append(includes, flags.proto.Dir.String())
			library.reexportDirs(includes...)
			library.reexportDeps(library.baseCompiler.pathDeps...) // TODO: restrict to proto deps
		}
	}

	if library.baseCompiler.hasSrcExt(".sysprop") {
		dir := android.PathForModuleGen(ctx, "sysprop", "include").String()
		if library.Properties.Sysprop.Platform != nil {
			isProduct := ctx.ProductSpecific() && !ctx.useVndk()
			isVendor := ctx.useVndk()
			isOwnerPlatform := Bool(library.Properties.Sysprop.Platform)

			if !ctx.inRecovery() && (isProduct || (isOwnerPlatform == isVendor)) {
				dir = android.PathForModuleGen(ctx, "sysprop/public", "include").String()
			}
		}

		library.reexportDirs(dir)
		library.reexportDeps(library.baseCompiler.pathDeps...)
	}

	if library.buildStubs() {
		library.reexportFlags("-D" + versioningMacroName(ctx.ModuleName()) + "=" + library.stubsVersion())
	}

	return out
}

func (library *libraryDecorator) buildStatic() bool {
	return library.MutatedProperties.BuildStatic &&
		BoolDefault(library.StaticProperties.Static.Enabled, true)
}

func (library *libraryDecorator) buildShared() bool {
	return library.MutatedProperties.BuildShared &&
		BoolDefault(library.SharedProperties.Shared.Enabled, true)
}

func (library *libraryDecorator) getWholeStaticMissingDeps() []string {
	return append([]string(nil), library.wholeStaticMissingDeps...)
}

func (library *libraryDecorator) objs() Objects {
	return library.objects
}

func (library *libraryDecorator) reuseObjs() (Objects, exportedFlagsProducer) {
	return library.reuseObjects, &library.flagExporter
}

func (library *libraryDecorator) toc() android.OptionalPath {
	return library.tocFile
}

func (library *libraryDecorator) installSymlinkToRuntimeApex(ctx ModuleContext, file android.Path) {
	dir := library.baseInstaller.installDir(ctx)
	dirOnDevice := android.InstallPathToOnDevicePath(ctx, dir)
	target := "/" + filepath.Join("apex", "com.android.runtime", dir.Base(), "bionic", file.Base())
	ctx.InstallAbsoluteSymlink(dir, file.Base(), target)
	library.post_install_cmds = append(library.post_install_cmds, makeSymlinkCmd(dirOnDevice, file.Base(), target))
}

func (library *libraryDecorator) install(ctx ModuleContext, file android.Path) {
	if library.shared() {
		if ctx.Device() && ctx.useVndk() {
			if ctx.isVndkSp() {
				library.baseInstaller.subDir = "vndk-sp"
			} else if ctx.isVndk() {
				if ctx.DeviceConfig().VndkUseCoreVariant() && !ctx.mustUseVendorVariant() {
					library.useCoreVariant = true
				}
				library.baseInstaller.subDir = "vndk"
			}

			// Append a version to vndk or vndk-sp directories on the system partition.
			if ctx.isVndk() && !ctx.isVndkExt() {
				vndkVersion := ctx.DeviceConfig().PlatformVndkVersion()
				if vndkVersion != "current" && vndkVersion != "" {
					library.baseInstaller.subDir += "-" + vndkVersion
				}
			}
		} else if len(library.Properties.Stubs.Versions) > 0 && android.DirectlyInAnyApex(ctx, ctx.ModuleName()) {
			// Bionic libraries (e.g. libc.so) is installed to the bootstrap subdirectory.
			// The original path becomes a symlink to the corresponding file in the
			// runtime APEX.
			translatedArch := ctx.Target().NativeBridge == android.NativeBridgeEnabled
			if InstallToBootstrap(ctx.baseModuleName(), ctx.Config()) && !library.buildStubs() && !translatedArch && !ctx.inRecovery() {
				if ctx.Device() {
					library.installSymlinkToRuntimeApex(ctx, file)
				}
				library.baseInstaller.subDir = "bootstrap"
			}
		} else if android.DirectlyInAnyApex(ctx, ctx.ModuleName()) && ctx.isLlndk(ctx.Config()) && !isBionic(ctx.baseModuleName()) {
			// Skip installing LLNDK (non-bionic) libraries moved to APEX.
			ctx.Module().SkipInstall()
		}

		library.baseInstaller.install(ctx, file)
	}

	if Bool(library.Properties.Static_ndk_lib) && library.static() &&
		!ctx.useVndk() && !ctx.inRecovery() && ctx.Device() &&
		library.baseLinker.sanitize.isUnsanitizedVariant() &&
		!library.buildStubs() {
		installPath := getNdkSysrootBase(ctx).Join(
			ctx, "usr/lib", config.NDKTriple(ctx.toolchain()), file.Base())

		ctx.ModuleBuild(pctx, android.ModuleBuildParams{
			Rule:        android.Cp,
			Description: "install " + installPath.Base(),
			Output:      installPath,
			Input:       file,
		})

		library.ndkSysrootPath = installPath
	}
}

func (library *libraryDecorator) static() bool {
	return library.MutatedProperties.VariantIsStatic
}

func (library *libraryDecorator) shared() bool {
	return library.MutatedProperties.VariantIsShared
}

func (library *libraryDecorator) header() bool {
	return !library.static() && !library.shared()
}

func (library *libraryDecorator) setStatic() {
	library.MutatedProperties.VariantIsStatic = true
	library.MutatedProperties.VariantIsShared = false
}

func (library *libraryDecorator) setShared() {
	library.MutatedProperties.VariantIsStatic = false
	library.MutatedProperties.VariantIsShared = true
}

func (library *libraryDecorator) BuildOnlyStatic() {
	library.MutatedProperties.BuildShared = false
}

func (library *libraryDecorator) BuildOnlyShared() {
	library.MutatedProperties.BuildStatic = false
}

func (library *libraryDecorator) HeaderOnly() {
	library.MutatedProperties.BuildShared = false
	library.MutatedProperties.BuildStatic = false
}

func (library *libraryDecorator) buildStubs() bool {
	return library.MutatedProperties.BuildStubs
}

func (library *libraryDecorator) symbolFileForAbiCheck(ctx ModuleContext) *string {
	if library.Properties.Header_abi_checker.Symbol_file != nil {
		return library.Properties.Header_abi_checker.Symbol_file
	}
	if ctx.hasStubsVariants() && library.Properties.Stubs.Symbol_file != nil {
		return library.Properties.Stubs.Symbol_file
	}
	return nil
}

func (library *libraryDecorator) stubsVersion() string {
	return library.MutatedProperties.StubsVersion
}

func (library *libraryDecorator) availableFor(what string) bool {
	var list []string
	if library.static() {
		list = library.StaticProperties.Static.Apex_available
	} else if library.shared() {
		list = library.SharedProperties.Shared.Apex_available
	}
	if len(list) == 0 {
		return false
	}
	return android.CheckAvailableForApex(what, list)
}

var versioningMacroNamesListKey = android.NewOnceKey("versioningMacroNamesList")

func versioningMacroNamesList(config android.Config) *map[string]string {
	return config.Once(versioningMacroNamesListKey, func() interface{} {
		m := make(map[string]string)
		return &m
	}).(*map[string]string)
}

// alphanumeric and _ characters are preserved.
// other characters are all converted to _
var charsNotForMacro = regexp.MustCompile("[^a-zA-Z0-9_]+")

func versioningMacroName(moduleName string) string {
	macroName := charsNotForMacro.ReplaceAllString(moduleName, "_")
	macroName = strings.ToUpper(moduleName)
	return "__" + macroName + "_API__"
}

func NewLibrary(hod android.HostOrDeviceSupported) (*Module, *libraryDecorator) {
	module := newModule(hod, android.MultilibBoth)

	library := &libraryDecorator{
		MutatedProperties: LibraryMutatedProperties{
			BuildShared: true,
			BuildStatic: true,
		},
		baseCompiler:  NewBaseCompiler(),
		baseLinker:    NewBaseLinker(module.sanitize),
		baseInstaller: NewBaseInstaller("lib", "lib64", InstallInSystem),
		sabi:          module.sabi,
	}

	module.compiler = library
	module.linker = library
	module.installer = library

	return module, library
}

// connects a shared library to a static library in order to reuse its .o files to avoid
// compiling source files twice.
func reuseStaticLibrary(mctx android.BottomUpMutatorContext, static, shared *Module) {
	if staticCompiler, ok := static.compiler.(*libraryDecorator); ok {
		sharedCompiler := shared.compiler.(*libraryDecorator)

		// Check libraries in addition to cflags, since libraries may be exporting different
		// include directories.
		if len(staticCompiler.StaticProperties.Static.Cflags) == 0 &&
			len(sharedCompiler.SharedProperties.Shared.Cflags) == 0 &&
			len(staticCompiler.StaticProperties.Static.Whole_static_libs) == 0 &&
			len(sharedCompiler.SharedProperties.Shared.Whole_static_libs) == 0 &&
			len(staticCompiler.StaticProperties.Static.Static_libs) == 0 &&
			len(sharedCompiler.SharedProperties.Shared.Static_libs) == 0 &&
			len(staticCompiler.StaticProperties.Static.Shared_libs) == 0 &&
			len(sharedCompiler.SharedProperties.Shared.Shared_libs) == 0 &&
			staticCompiler.StaticProperties.Static.System_shared_libs == nil &&
			sharedCompiler.SharedProperties.Shared.System_shared_libs == nil {

			mctx.AddInterVariantDependency(reuseObjTag, shared, static)
			sharedCompiler.baseCompiler.Properties.OriginalSrcs =
				sharedCompiler.baseCompiler.Properties.Srcs
			sharedCompiler.baseCompiler.Properties.Srcs = nil
			sharedCompiler.baseCompiler.Properties.Generated_sources = nil
		} else {
			// This dep is just to reference static variant from shared variant
			mctx.AddInterVariantDependency(staticVariantTag, shared, static)
		}
	}
}

func LinkageMutator(mctx android.BottomUpMutatorContext) {
	if m, ok := mctx.Module().(*Module); ok && m.linker != nil {
		switch library := m.linker.(type) {
		case prebuiltLibraryInterface:
			// Always create both the static and shared variants for prebuilt libraries, and then disable the one
			// that is not being used.  This allows them to share the name of a cc_library module, which requires that
			// all the variants of the cc_library also exist on the prebuilt.
			modules := mctx.CreateLocalVariations("static", "shared")
			static := modules[0].(*Module)
			shared := modules[1].(*Module)

			static.linker.(prebuiltLibraryInterface).setStatic()
			shared.linker.(prebuiltLibraryInterface).setShared()

			if !library.buildStatic() {
				static.linker.(prebuiltLibraryInterface).disablePrebuilt()
			}
			if !library.buildShared() {
				shared.linker.(prebuiltLibraryInterface).disablePrebuilt()
			}

		case libraryInterface:
			if library.buildStatic() && library.buildShared() {
				modules := mctx.CreateLocalVariations("static", "shared")
				static := modules[0].(*Module)
				shared := modules[1].(*Module)

				static.linker.(libraryInterface).setStatic()
				shared.linker.(libraryInterface).setShared()

				reuseStaticLibrary(mctx, static, shared)

			} else if library.buildStatic() {
				modules := mctx.CreateLocalVariations("static")
				modules[0].(*Module).linker.(libraryInterface).setStatic()
			} else if library.buildShared() {
				modules := mctx.CreateLocalVariations("shared")
				modules[0].(*Module).linker.(libraryInterface).setShared()
			}
		}
	}
}

var stubVersionsKey = android.NewOnceKey("stubVersions")

// maps a module name to the list of stubs versions available for the module
func stubsVersionsFor(config android.Config) map[string][]string {
	return config.Once(stubVersionsKey, func() interface{} {
		return make(map[string][]string)
	}).(map[string][]string)
}

var stubsVersionsLock sync.Mutex

func latestStubsVersionFor(config android.Config, name string) string {
	versions, ok := stubsVersionsFor(config)[name]
	if ok && len(versions) > 0 {
		// the versions are alreay sorted in ascending order
		return versions[len(versions)-1]
	}
	return ""
}

// Version mutator splits a module into the mandatory non-stubs variant
// (which is unnamed) and zero or more stubs variants.
func VersionMutator(mctx android.BottomUpMutatorContext) {
	if m, ok := mctx.Module().(*Module); ok && !m.inRecovery() && m.linker != nil {
		if library, ok := m.linker.(*libraryDecorator); ok && library.buildShared() &&
			len(library.Properties.Stubs.Versions) > 0 {
			versions := []string{}
			for _, v := range library.Properties.Stubs.Versions {
				if _, err := strconv.Atoi(v); err != nil {
					mctx.PropertyErrorf("versions", "%q is not a number", v)
				}
				versions = append(versions, v)
			}
			sort.Slice(versions, func(i, j int) bool {
				left, _ := strconv.Atoi(versions[i])
				right, _ := strconv.Atoi(versions[j])
				return left < right
			})

			// save the list of versions for later use
			copiedVersions := make([]string, len(versions))
			copy(copiedVersions, versions)
			stubsVersionsLock.Lock()
			defer stubsVersionsLock.Unlock()
			stubsVersionsFor(mctx.Config())[mctx.ModuleName()] = copiedVersions

			// "" is for the non-stubs variant
			versions = append([]string{""}, versions...)

			modules := mctx.CreateVariations(versions...)
			for i, m := range modules {
				l := m.(*Module).linker.(*libraryDecorator)
				if versions[i] != "" {
					l.MutatedProperties.BuildStubs = true
					l.MutatedProperties.StubsVersion = versions[i]
					m.(*Module).Properties.HideFromMake = true
					m.(*Module).sanitize = nil
					m.(*Module).stl = nil
					m.(*Module).Properties.PreventInstall = true
				}
			}
		} else {
			mctx.CreateVariations("")
		}
		return
	}
	if genrule, ok := mctx.Module().(*genrule.Module); ok {
		if props, ok := genrule.Extra.(*GenruleExtraProperties); ok && !props.InRecovery {
			mctx.CreateVariations("")
			return
		}
	}
}

// maybeInjectBoringSSLHash adds a rule to run bssl_inject_hash on the output file if the module has the
// inject_bssl_hash or if any static library dependencies have inject_bssl_hash set.  It returns the output path
// that the linked output file should be written to.
// TODO(b/137267623): Remove this in favor of a cc_genrule when they support operating on shared libraries.
func maybeInjectBoringSSLHash(ctx android.ModuleContext, outputFile android.ModuleOutPath,
	inject *bool, fileName string) android.ModuleOutPath {
	// TODO(b/137267623): Remove this in favor of a cc_genrule when they support operating on shared libraries.
	injectBoringSSLHash := Bool(inject)
	ctx.VisitDirectDeps(func(dep android.Module) {
		tag := ctx.OtherModuleDependencyTag(dep)
		if tag == staticDepTag || tag == staticExportDepTag || tag == wholeStaticDepTag || tag == lateStaticDepTag {
			if cc, ok := dep.(*Module); ok {
				if library, ok := cc.linker.(*libraryDecorator); ok {
					if Bool(library.Properties.Inject_bssl_hash) {
						injectBoringSSLHash = true
					}
				}
			}
		}
	})
	if injectBoringSSLHash {
		hashedOutputfile := outputFile
		outputFile = android.PathForModuleOut(ctx, "unhashed", fileName)

		rule := android.NewRuleBuilder()
		rule.Command().
			BuiltTool(ctx, "bssl_inject_hash").
			Flag("-sha256").
			FlagWithInput("-in-object ", outputFile).
			FlagWithOutput("-o ", hashedOutputfile)
		rule.Build(pctx, ctx, "injectCryptoHash", "inject crypto hash")
	}

	return outputFile
}
