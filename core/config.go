// Copyright 2014 nps Author. All Rights Reserved.
package core

import "regexp"

// this structure is used to describe the plugin configuration item name and description.
type Config struct {
	ConfigName    string         // single configuration item name
	ZhTitle       string         // single configuration item chinese title
	EnTitle       string         // single configuration item english title
	ZhDescription string         // single configuration item chinese description
	EnDescription string         // single configuration item english description
	LimitReg      *regexp.Regexp // regular expression to restrict input
	ConfigLevel   ConfigLevel    // configuration sector
}

// multiple configuration collections for plugins
type NpsConfigs struct {
	ZhTitle       string    // chinese title for configuration collection
	EnTitle       string    // chinese description of the configuration collection
	EnDescription string    // english description of the configuration collection
	ZhDescription string    // chinese description for english collection
	configs       []*Config // all configurations
}

// insert one config into configs
func (config *NpsConfigs) Add(cfg *Config) {
	config.configs = append(config.configs, cfg)
}

// get all configs
func (config *NpsConfigs) GetAll() []*Config {
	return config.configs
}
