package core

// This structure is used to describe the plugin configuration item name and description.
type Config struct {
	ConfigName  string
	Description string
}

type NpsConfigs struct {
	configs []*Config
}

func NewNpsConfigs(name, des string) *NpsConfigs {
	c := &NpsConfigs{}
	c.configs = make([]*Config, 0)
	c.Add(name, des)
	return c
}

func (config *NpsConfigs) Add(name, des string) {
	config.configs = append(config.configs, &Config{ConfigName: name, Description: des})
}

func (config *NpsConfigs) GetAll() []*Config {
	return config.configs
}
