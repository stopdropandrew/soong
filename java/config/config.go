// Copyright 2017 Google Inc. All rights reserved.
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

package config

import (
	"path/filepath"
	"runtime"
	"strings"

	_ "github.com/google/blueprint/bootstrap"

	"android/soong/android"
)

var (
	pctx = android.NewPackageContext("android/soong/java/config")

	DefaultBootclasspathLibraries = []string{"core.platform.api.stubs", "core-lambda-stubs"}
	DefaultSystemModules          = "core-platform-api-stubs-system-modules"
	DefaultLibraries              = []string{"ext", "framework", "updatable_media_stubs"}
	DefaultLambdaStubsLibrary     = "core-lambda-stubs"
	SdkLambdaStubsPath            = "prebuilts/sdk/tools/core-lambda-stubs.jar"

	DefaultJacocoExcludeFilter = []string{"org.junit.*", "org.jacoco.*", "org.mockito.*"}

	InstrumentFrameworkModules = []string{
		"framework",
		"telephony-common",
		"services",
		"android.car",
		"android.car7",
		"conscrypt",
		"core-icu4j",
		"core-oj",
		"core-libart",
		"updatable-media",
	}
)

const (
	JavaVmFlags  = `-XX:OnError="cat hs_err_pid%p.log" -XX:CICompilerCount=6 -XX:+UseDynamicNumberOfGCThreads`
	JavacVmFlags = `-J-XX:OnError="cat hs_err_pid%p.log" -J-XX:CICompilerCount=6 -J-XX:+UseDynamicNumberOfGCThreads`
)

func init() {
	pctx.Import("github.com/google/blueprint/bootstrap")

	pctx.StaticVariable("JavacHeapSize", "2048M")
	pctx.StaticVariable("JavacHeapFlags", "-J-Xmx${JavacHeapSize}")
	pctx.StaticVariable("DexFlags", "-JXX:OnError='cat hs_err_pid%p.log' -JXX:CICompilerCount=6 -JXX:+UseDynamicNumberOfGCThreads")

	pctx.StaticVariable("CommonJdkFlags", strings.Join([]string{
		`-Xmaxerrs 9999999`,
		`-encoding UTF-8`,
		`-sourcepath ""`,
		`-g`,
		// Turbine leaves out bridges which can cause javac to unnecessarily insert them into
		// subclasses (b/65645120).  Setting this flag causes our custom javac to assume that
		// the missing bridges will exist at runtime and not recreate them in subclasses.
		// If a different javac is used the flag will be ignored and extra bridges will be inserted.
		// The flag is implemented by https://android-review.googlesource.com/c/486427
		`-XDskipDuplicateBridges=true`,

		// b/65004097: prevent using java.lang.invoke.StringConcatFactory when using -target 1.9
		`-XDstringConcat=inline`,
	}, " "))

	pctx.StaticVariable("JavaVmFlags", JavaVmFlags)
	pctx.StaticVariable("JavacVmFlags", JavacVmFlags)

	pctx.VariableConfigMethod("hostPrebuiltTag", android.Config.PrebuiltOS)

	pctx.VariableFunc("JavaHome", func(ctx android.PackageVarContext) string {
		// This is set up and guaranteed by soong_ui
		return ctx.Config().Getenv("ANDROID_JAVA_HOME")
	})
	pctx.VariableFunc("JlinkVersion", func(ctx android.PackageVarContext) string {
		switch ctx.Config().Getenv("EXPERIMENTAL_USE_OPENJDK11_TOOLCHAIN") {
		case "true":
			return "11"
		default:
			return "9"
		}
	})

	pctx.SourcePathVariable("JavaToolchain", "${JavaHome}/bin")
	pctx.SourcePathVariableWithEnvOverride("JavacCmd",
		"${JavaToolchain}/javac", "ALTERNATE_JAVAC")
	pctx.SourcePathVariable("JavaCmd", "${JavaToolchain}/java")
	pctx.SourcePathVariable("JarCmd", "${JavaToolchain}/jar")
	pctx.SourcePathVariable("JavadocCmd", "${JavaToolchain}/javadoc")
	pctx.SourcePathVariable("JlinkCmd", "${JavaToolchain}/jlink")
	pctx.SourcePathVariable("JmodCmd", "${JavaToolchain}/jmod")
	pctx.SourcePathVariable("JrtFsJar", "${JavaHome}/lib/jrt-fs.jar")
	pctx.SourcePathVariable("JavaKytheExtractorJar", "prebuilts/build-tools/common/framework/javac_extractor.jar")
	pctx.SourcePathVariable("Ziptime", "prebuilts/build-tools/${hostPrebuiltTag}/bin/ziptime")

	pctx.SourcePathVariable("GenKotlinBuildFileCmd", "build/soong/scripts/gen-kotlin-build-file.sh")

	pctx.SourcePathVariable("JarArgsCmd", "build/soong/scripts/jar-args.sh")
	pctx.SourcePathVariable("PackageCheckCmd", "build/soong/scripts/package-check.sh")
	pctx.HostBinToolVariable("ExtractJarPackagesCmd", "extract_jar_packages")
	pctx.HostBinToolVariable("SoongZipCmd", "soong_zip")
	pctx.HostBinToolVariable("MergeZipsCmd", "merge_zips")
	pctx.HostBinToolVariable("Zip2ZipCmd", "zip2zip")
	pctx.HostBinToolVariable("ZipSyncCmd", "zipsync")
	pctx.HostBinToolVariable("ApiCheckCmd", "apicheck")
	pctx.HostBinToolVariable("D8Cmd", "d8")
	pctx.HostBinToolVariable("R8Cmd", "r8-compat-proguard")
	pctx.HostBinToolVariable("HiddenAPICmd", "hiddenapi")

	pctx.VariableFunc("TurbineJar", func(ctx android.PackageVarContext) string {
		turbine := "turbine.jar"
		if ctx.Config().UnbundledBuild() {
			return "prebuilts/build-tools/common/framework/" + turbine
		} else {
			return pctx.HostJavaToolPath(ctx, turbine).String()
		}
	})

	pctx.HostJavaToolVariable("JarjarCmd", "jarjar.jar")
	pctx.HostJavaToolVariable("JsilverJar", "jsilver.jar")
	pctx.HostJavaToolVariable("DoclavaJar", "doclava.jar")
	pctx.HostJavaToolVariable("MetalavaJar", "metalava.jar")
	pctx.HostJavaToolVariable("DokkaJar", "dokka.jar")
	pctx.HostJavaToolVariable("JetifierJar", "jetifier.jar")

	pctx.HostBinToolVariable("SoongJavacWrapper", "soong_javac_wrapper")
	pctx.HostBinToolVariable("DexpreoptGen", "dexpreopt_gen")

	pctx.VariableFunc("JavacWrapper", func(ctx android.PackageVarContext) string {
		if override := ctx.Config().Getenv("JAVAC_WRAPPER"); override != "" {
			return override + " "
		}
		return ""
	})

	pctx.HostJavaToolVariable("JacocoCLIJar", "jacoco-cli.jar")

	pctx.HostBinToolVariable("ManifestCheckCmd", "manifest_check")
	pctx.HostBinToolVariable("ManifestFixerCmd", "manifest_fixer")

	pctx.HostBinToolVariable("ManifestMergerCmd", "manifest-merger")

	pctx.HostBinToolVariable("Class2Greylist", "class2greylist")
	pctx.HostBinToolVariable("HiddenAPI", "hiddenapi")

	hostBinToolVariableWithSdkToolsPrebuilt("Aapt2Cmd", "aapt2")
	hostBinToolVariableWithBuildToolsPrebuilt("AidlCmd", "aidl")
	hostBinToolVariableWithBuildToolsPrebuilt("ZipAlign", "zipalign")

	hostJavaToolVariableWithSdkToolsPrebuilt("SignapkCmd", "signapk")
	// TODO(ccross): this should come from the signapk dependencies, but we don't have any way
	// to express host JNI dependencies yet.
	hostJNIToolVariableWithSdkToolsPrebuilt("SignapkJniLibrary", "libconscrypt_openjdk_jni")
}

func hostBinToolVariableWithSdkToolsPrebuilt(name, tool string) {
	pctx.VariableFunc(name, func(ctx android.PackageVarContext) string {
		if ctx.Config().UnbundledBuild() || ctx.Config().IsPdkBuild() {
			return filepath.Join("prebuilts/sdk/tools", runtime.GOOS, "bin", tool)
		} else {
			return pctx.HostBinToolPath(ctx, tool).String()
		}
	})
}

func hostJavaToolVariableWithSdkToolsPrebuilt(name, tool string) {
	pctx.VariableFunc(name, func(ctx android.PackageVarContext) string {
		if ctx.Config().UnbundledBuild() || ctx.Config().IsPdkBuild() {
			return filepath.Join("prebuilts/sdk/tools/lib", tool+".jar")
		} else {
			return pctx.HostJavaToolPath(ctx, tool+".jar").String()
		}
	})
}

func hostJNIToolVariableWithSdkToolsPrebuilt(name, tool string) {
	pctx.VariableFunc(name, func(ctx android.PackageVarContext) string {
		if ctx.Config().UnbundledBuild() || ctx.Config().IsPdkBuild() {
			ext := ".so"
			if runtime.GOOS == "darwin" {
				ext = ".dylib"
			}
			return filepath.Join("prebuilts/sdk/tools", runtime.GOOS, "lib64", tool+ext)
		} else {
			return pctx.HostJNIToolPath(ctx, tool).String()
		}
	})
}

func hostBinToolVariableWithBuildToolsPrebuilt(name, tool string) {
	pctx.VariableFunc(name, func(ctx android.PackageVarContext) string {
		if ctx.Config().UnbundledBuild() || ctx.Config().IsPdkBuild() {
			return filepath.Join("prebuilts/build-tools", ctx.Config().PrebuiltOS(), "bin", tool)
		} else {
			return pctx.HostBinToolPath(ctx, tool).String()
		}
	})
}

// JavaCmd returns a SourcePath object with the path to the java command.
func JavaCmd(ctx android.PathContext) android.SourcePath {
	return javaTool(ctx, "java")
}

// JavadocCmd returns a SourcePath object with the path to the java command.
func JavadocCmd(ctx android.PathContext) android.SourcePath {
	return javaTool(ctx, "javadoc")
}

func javaTool(ctx android.PathContext, tool string) android.SourcePath {
	type javaToolKey string

	key := android.NewCustomOnceKey(javaToolKey(tool))

	return ctx.Config().OnceSourcePath(key, func() android.SourcePath {
		return javaToolchain(ctx).Join(ctx, tool)
	})

}

var javaToolchainKey = android.NewOnceKey("javaToolchain")

func javaToolchain(ctx android.PathContext) android.SourcePath {
	return ctx.Config().OnceSourcePath(javaToolchainKey, func() android.SourcePath {
		return javaHome(ctx).Join(ctx, "bin")
	})
}

var javaHomeKey = android.NewOnceKey("javaHome")

func javaHome(ctx android.PathContext) android.SourcePath {
	return ctx.Config().OnceSourcePath(javaHomeKey, func() android.SourcePath {
		// This is set up and guaranteed by soong_ui
		return android.PathForSource(ctx, ctx.Config().Getenv("ANDROID_JAVA_HOME"))
	})
}
