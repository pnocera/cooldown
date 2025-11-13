package config

import (
	"fmt"
	"gopkg.in/yaml.v3"
	"os"
	"regexp"
)

func Load(configPath string) (*Config, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	return LoadFromYAMLBytes(data)
}

func LoadFromYAMLString(yamlContent string) (*Config, error) {
	return LoadFromYAMLBytes([]byte(yamlContent))
}

func LoadFromYAMLBytes(data []byte) (*Config, error) {
	var config Config
	err := yaml.Unmarshal(data, &config)
	if err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	// Set defaults
	if config.Server.Host == "" {
		config.Server.Host = "localhost"
	}
	if config.Server.Port == 0 {
		config.Server.Port = 8080
	}

	// Set CerebrasLimits defaults
	config.CerebrasLimits.SetDefaults()

	// Expand environment variables
	expandEnvironmentVariablesInConfig(&config)

	return &config, nil
}

var envVarRegex = regexp.MustCompile(`\$\{([^:}]+):?([^}]*)\}`)

func expandEnvironmentVariables(value string) string {
	return envVarRegex.ReplaceAllStringFunc(value, func(match string) string {
		matches := envVarRegex.FindStringSubmatch(match)
		if len(matches) < 2 {
			return match
		}

		envKey := matches[1]
		defaultValue := ""
		if len(matches) > 2 {
			defaultValue = matches[2]
		}

		if envValue := os.Getenv(envKey); envValue != "" {
			return envValue
		}
		return defaultValue
	})
}

func expandEnvironmentVariablesInConfig(config *Config) {
	// Expand environment models
	config.EnvironmentModels.Haiku = expandEnvironmentVariables(config.EnvironmentModels.Haiku)
	config.EnvironmentModels.Sonnet = expandEnvironmentVariables(config.EnvironmentModels.Sonnet)
	config.EnvironmentModels.Opus = expandEnvironmentVariables(config.EnvironmentModels.Opus)

	// Expand provider configurations
	for i := range config.Providers {
		config.Providers[i].Endpoint = expandEnvironmentVariables(config.Providers[i].Endpoint)
		config.Providers[i].APIKey = expandEnvironmentVariables(config.Providers[i].APIKey)

		// Check if LoadBalancing is not nil before accessing APIKeys
		if config.Providers[i].LoadBalancing != nil {
			for j := range config.Providers[i].LoadBalancing.APIKeys {
				config.Providers[i].LoadBalancing.APIKeys[j].Key = expandEnvironmentVariables(
					config.Providers[i].LoadBalancing.APIKeys[j].Key)
			}
		}
	}
}
