package core

// This structure is used to describe the plugin configuration item name and description.
type Config struct {
	ConfigName  string
	Description string
	ConfigLevel ConfigLevel
}

type NpsConfigs struct {
	configs []*Config
}

func NewNpsConfigs(name, des string, level ConfigLevel) *NpsConfigs {
	c := &NpsConfigs{}
	c.configs = make([]*Config, 0)
	c.Add(name, des, level)
	return c
}

func (config *NpsConfigs) Add(name, des string, level ConfigLevel) {
	config.configs = append(config.configs, &Config{ConfigName: name, Description: des, ConfigLevel: level})
}

func (config *NpsConfigs) GetAll() []*Config {
	return config.configs
}
