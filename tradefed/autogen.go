// Copyright 2018 Google Inc. All rights reserved.
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

package tradefed

import (
	"fmt"
	"strings"

	"github.com/google/blueprint"
	"github.com/google/blueprint/proptools"

	"android/soong/android"
)

const test_xml_indent = "    "

func getTestConfigTemplate(ctx android.ModuleContext, prop *string) android.OptionalPath {
	return ctx.ExpandOptionalSource(prop, "test_config_template")
}

func getTestConfig(ctx android.ModuleContext, prop *string) android.Path {
	if p := ctx.ExpandOptionalSource(prop, "test_config"); p.Valid() {
		return p.Path()
	} else if p := android.ExistentPathForSource(ctx, ctx.ModuleDir(), "AndroidTest.xml"); p.Valid() {
		return p.Path()
	}
	return nil
}

var autogenTestConfig = pctx.StaticRule("autogenTestConfig", blueprint.RuleParams{
	Command:     "sed 's&{MODULE}&${name}&g;s&{EXTRA_CONFIGS}&'${extraConfigs}'&g' $template > $out",
	CommandDeps: []string{"$template"},
}, "name", "template", "extraConfigs")

func testConfigPath(ctx android.ModuleContext, prop *string, testSuites []string, autoGenConfig *bool) (path android.Path, autogenPath android.WritablePath) {
	p := getTestConfig(ctx, prop)
	if !Bool(autoGenConfig) && p != nil {
		return p, nil
	} else if !android.InList("cts", testSuites) && BoolDefault(autoGenConfig, true) {
		outputFile := android.PathForModuleOut(ctx, ctx.ModuleName()+".config")
		return nil, outputFile
	} else {
		// CTS modules can be used for test data, so test config files must be
		// explicitly created using AndroidTest.xml
		// TODO(b/112602712): remove the path check
		return nil, nil
	}
}

type Config interface {
	Config() string
}

type Option struct {
	Name  string
	Value string
}

var _ Config = Option{}

func (o Option) Config() string {
	return fmt.Sprintf(`<option name="%s" value="%s" />`, o.Name, o.Value)
}

// It can be a template of object or target_preparer.
type Object struct {
	// Set it as a target_preparer if object type == "target_preparer".
	Type    string
	Class   string
	Options []Option
}

var _ Config = Object{}

func (ob Object) Config() string {
	var optionStrings []string
	for _, option := range ob.Options {
		optionStrings = append(optionStrings, option.Config())
	}
	var options string
	if len(ob.Options) == 0 {
		options = ""
	} else {
		optionDelimiter := fmt.Sprintf("\\n%s%s", test_xml_indent, test_xml_indent)
		options = optionDelimiter + strings.Join(optionStrings, optionDelimiter)
	}
	if ob.Type == "target_preparer" {
		return fmt.Sprintf(`<target_preparer class="%s">%s\n%s</target_preparer>`, ob.Class, options, test_xml_indent)
	} else {
		return fmt.Sprintf(`<object type="%s" class="%s">%s\n%s</object>`, ob.Type, ob.Class, options, test_xml_indent)
	}

}

func autogenTemplate(ctx android.ModuleContext, output android.WritablePath, template string, configs []Config) {
	var configStrings []string
	for _, config := range configs {
		configStrings = append(configStrings, config.Config())
	}
	extraConfigs := strings.Join(configStrings, fmt.Sprintf("\\n%s", test_xml_indent))
	extraConfigs = proptools.NinjaAndShellEscape(extraConfigs)

	ctx.Build(pctx, android.BuildParams{
		Rule:        autogenTestConfig,
		Description: "test config",
		Output:      output,
		Args: map[string]string{
			"name":         ctx.ModuleName(),
			"template":     template,
			"extraConfigs": extraConfigs,
		},
	})
}

func AutoGenNativeTestConfig(ctx android.ModuleContext, testConfigProp *string,
	testConfigTemplateProp *string, testSuites []string, config []Config, autoGenConfig *bool) android.Path {
	path, autogenPath := testConfigPath(ctx, testConfigProp, testSuites, autoGenConfig)
	if autogenPath != nil {
		templatePath := getTestConfigTemplate(ctx, testConfigTemplateProp)
		if templatePath.Valid() {
			autogenTemplate(ctx, autogenPath, templatePath.String(), config)
		} else {
			if ctx.Device() {
				autogenTemplate(ctx, autogenPath, "${NativeTestConfigTemplate}", config)
			} else {
				autogenTemplate(ctx, autogenPath, "${NativeHostTestConfigTemplate}", config)
			}
		}
		return autogenPath
	}
	return path
}

func AutoGenNativeBenchmarkTestConfig(ctx android.ModuleContext, testConfigProp *string,
	testConfigTemplateProp *string, testSuites []string, configs []Config, autoGenConfig *bool) android.Path {
	path, autogenPath := testConfigPath(ctx, testConfigProp, testSuites, autoGenConfig)
	if autogenPath != nil {
		templatePath := getTestConfigTemplate(ctx, testConfigTemplateProp)
		if templatePath.Valid() {
			autogenTemplate(ctx, autogenPath, templatePath.String(), configs)
		} else {
			autogenTemplate(ctx, autogenPath, "${NativeBenchmarkTestConfigTemplate}", configs)
		}
		return autogenPath
	}
	return path
}

func AutoGenJavaTestConfig(ctx android.ModuleContext, testConfigProp *string, testConfigTemplateProp *string,
	testSuites []string, autoGenConfig *bool) android.Path {
	path, autogenPath := testConfigPath(ctx, testConfigProp, testSuites, autoGenConfig)
	if autogenPath != nil {
		templatePath := getTestConfigTemplate(ctx, testConfigTemplateProp)
		if templatePath.Valid() {
			autogenTemplate(ctx, autogenPath, templatePath.String(), nil)
		} else {
			if ctx.Device() {
				autogenTemplate(ctx, autogenPath, "${JavaTestConfigTemplate}", nil)
			} else {
				autogenTemplate(ctx, autogenPath, "${JavaHostTestConfigTemplate}", nil)
			}
		}
		return autogenPath
	}
	return path
}

func AutoGenPythonBinaryHostTestConfig(ctx android.ModuleContext, testConfigProp *string,
	testConfigTemplateProp *string, testSuites []string, autoGenConfig *bool) android.Path {

	path, autogenPath := testConfigPath(ctx, testConfigProp, testSuites, autoGenConfig)
	if autogenPath != nil {
		templatePath := getTestConfigTemplate(ctx, testConfigTemplateProp)
		if templatePath.Valid() {
			autogenTemplate(ctx, autogenPath, templatePath.String(), nil)
		} else {
			autogenTemplate(ctx, autogenPath, "${PythonBinaryHostTestConfigTemplate}", nil)
		}
		return autogenPath
	}
	return path
}

var autogenInstrumentationTest = pctx.StaticRule("autogenInstrumentationTest", blueprint.RuleParams{
	Command: "${AutoGenTestConfigScript} $out $in ${EmptyTestConfig} $template",
	CommandDeps: []string{
		"${AutoGenTestConfigScript}",
		"${EmptyTestConfig}",
		"$template",
	},
}, "name", "template")

func AutoGenInstrumentationTestConfig(ctx android.ModuleContext, testConfigProp *string,
	testConfigTemplateProp *string, manifest android.Path, testSuites []string, autoGenConfig *bool) android.Path {
	path, autogenPath := testConfigPath(ctx, testConfigProp, testSuites, autoGenConfig)
	if autogenPath != nil {
		template := "${InstrumentationTestConfigTemplate}"
		moduleTemplate := getTestConfigTemplate(ctx, testConfigTemplateProp)
		if moduleTemplate.Valid() {
			template = moduleTemplate.String()
		}
		ctx.Build(pctx, android.BuildParams{
			Rule:        autogenInstrumentationTest,
			Description: "test config",
			Input:       manifest,
			Output:      autogenPath,
			Args: map[string]string{
				"name":     ctx.ModuleName(),
				"template": template,
			},
		})
		return autogenPath
	}
	return path
}

var Bool = proptools.Bool
var BoolDefault = proptools.BoolDefault
