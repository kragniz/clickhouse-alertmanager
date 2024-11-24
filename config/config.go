package config

import (
	"os"

	"github.com/goccy/go-yaml"
)

type Config struct {
	Clickhouse         Clickhouse `yaml:"clickhouse"`
	RuleFiles          []string   `yaml:"rule_files"`
	EvaluationInterval int        `yaml:"evaluation_interval"`
}

type Clickhouse struct {
	Addresses []string `yaml:"addresses"`
	Database  string   `yaml:"database"`
	Username  string   `yaml:"username"`
	Password  string   `yaml:"password"`
	TLS       bool     `yaml:"tls"`
}

func ReadConfig(filename string) (*Config, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var config Config
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		return nil, err
	}

	return &config, nil
}
