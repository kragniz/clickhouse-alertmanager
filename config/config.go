package config

import (
	"os"

	"github.com/goccy/go-yaml"
)

type AlertConfig struct {
	Groups []Group `yaml:"groups"`
}

type Group struct {
	Name   string            `yaml:"name"`
	Labels map[string]string `yaml:"labels"`
	Rules  []Rule            `yaml:"rules"`
}

type Rule struct {
	AlertName   string            `yaml:"alert"`
	Expr        string            `yaml:"expr"`
	Labels      map[string]string `yaml:"labels"`
	Annotations map[string]string `yaml:"annotations"`
}

func ReadAlertConfig(filename string) (*AlertConfig, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var config AlertConfig
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		return nil, err
	}

	return &config, nil
}
