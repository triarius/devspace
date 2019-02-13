package configutil

import (
	"io/ioutil"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/covexo/devspace/pkg/util/log"
	"github.com/covexo/devspace/pkg/util/ptr"
	"github.com/covexo/devspace/pkg/util/stdinutil"

	"github.com/covexo/devspace/pkg/devspace/config/configs"
	"github.com/covexo/devspace/pkg/devspace/config/generated"
	"github.com/covexo/devspace/pkg/devspace/config/versions"
	"github.com/covexo/devspace/pkg/devspace/config/versions/latest"
	"github.com/covexo/devspace/pkg/devspace/deploy/kubectl/walk"
	yaml "gopkg.in/yaml.v2"
)

// VarMatchRegex is the regex to check if a value matches the devspace var format
var VarMatchRegex = regexp.MustCompile("^\\$\\{[^\\}]+\\}$")

// VarEnvPrefix is the prefix environment variables should have in order to use them
const VarEnvPrefix = "DEVSPACE_VAR_"

func varReplaceFn(value string) interface{} {
	varName := strings.TrimSpace(value[2 : len(value)-1])
	retString := ""

	if os.Getenv(VarEnvPrefix+strings.ToUpper(varName)) != "" {
		retString = os.Getenv(VarEnvPrefix + strings.ToUpper(varName))

		// Check if we can convert val
		if retString == "true" {
			return true
		} else if retString == "false" {
			return false
		} else if i, err := strconv.Atoi(retString); err == nil {
			return i
		}

		return retString
	}

	generatedConfig, err := generated.LoadConfig()
	if err != nil {
		log.Fatalf("Error reading generated config: %v", err)
	}

	currentConfig := generatedConfig.GetActive()
	if configVal, ok := currentConfig.Vars[value]; ok {
		return configVal
	}

	currentConfig.Vars[varName] = AskQuestion(&configs.Variable{
		Question: ptr.String("Please enter a value for " + varName),
	})

	err = generated.SaveConfig(generatedConfig)
	if err != nil {
		log.Fatalf("Error saving generated config: %v", err)
	}

	return currentConfig.Vars[value]
}

func varMatchFn(key, value string) bool {
	return VarMatchRegex.MatchString(value)
}

// AskQuestion asks the user a question depending on the variable options
func AskQuestion(variable *configs.Variable) interface{} {
	params := &stdinutil.GetFromStdinParams{}

	if variable == nil {
		params.Question = "Please enter a value"
	} else {
		if variable.Question == nil {
			params.Question = "Please enter a value"
		} else {
			params.Question = *variable.Question
		}

		if variable.Default != nil {
			params.DefaultValue = *variable.Default
		}
		if variable.RegexPattern != nil {
			params.ValidationRegexPattern = *variable.RegexPattern
		}
	}

	configVal := *stdinutil.GetFromStdin(params)

	// Check if we can convert configVal
	if configVal == "true" {
		return true
	} else if configVal == "false" {
		return false
	} else if i, err := strconv.Atoi(configVal); err == nil {
		return i
	}

	return configVal
}

func loadConfigFromPath(path string) (*latest.Config, error) {
	yamlFileContent, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	out, err := resolveVars(yamlFileContent)
	if err != nil {
		return nil, err
	}

	oldConfig := map[interface{}]interface{}{}
	err = yaml.Unmarshal(out, oldConfig)
	if err != nil {
		return nil, err
	}

	newConfig, err := versions.Parse(oldConfig)
	if err != nil {
		return nil, err
	}

	return newConfig, nil
}

func loadConfigFromInterface(m map[interface{}]interface{}) (*latest.Config, error) {
	yamlFileContent, err := yaml.Marshal(m)
	if err != nil {
		return nil, err
	}

	out, err := resolveVars(yamlFileContent)
	if err != nil {
		return nil, err
	}

	oldConfig := map[interface{}]interface{}{}
	err = yaml.Unmarshal(out, oldConfig)
	if err != nil {
		return nil, err
	}

	newConfig, err := versions.Parse(oldConfig)
	if err != nil {
		return nil, err
	}

	return newConfig, nil
}

// LoadConfigs loads all the configs from the .devspace/configs.yaml
func LoadConfigs(configs *configs.Configs, path string) error {
	yamlFileContent, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}

	return yaml.UnmarshalStrict(yamlFileContent, configs)
}

func resolveVars(yamlFileContent []byte) ([]byte, error) {
	rawConfig := make(map[interface{}]interface{})

	err := yaml.Unmarshal(yamlFileContent, &rawConfig)
	if err != nil {
		return nil, err
	}

	walk.Walk(rawConfig, varMatchFn, varReplaceFn)

	out, err := yaml.Marshal(rawConfig)
	if err != nil {
		return nil, err
	}

	return out, nil
}
