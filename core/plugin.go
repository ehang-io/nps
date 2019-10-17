package core

import (
	"context"
	"github.com/cnlh/nps/bridge"
	"net"
)

// Plugin interface, all plugins must implement those functions.
type Plugin interface {
	GetConfigName() *NpsConfigs
	InitConfig(globalConfig, clientConfig, pluginConfig map[string]string, pgCnf []*Config)
	GetStage() []Stage
	Start(ctx context.Context) (context.Context, error)
	Run(ctx context.Context) (context.Context, error)
	End(ctx context.Context) (context.Context, error)
}

type NpsPlugin struct {
	Version string
	Configs map[string]string
}

func (npsPlugin *NpsPlugin) GetConfigName() *NpsConfigs {
	return nil
}

func (npsPlugin *NpsPlugin) InitConfig(globalConfig, clientConfig, pluginConfig map[string]string, pgCnf []*Config) {
	npsPlugin.Configs = make(map[string]string)
	for _, cfg := range pgCnf {
		switch cfg.ConfigLevel {
		case CONFIG_LEVEL_PLUGIN:
			npsPlugin.Configs[cfg.ConfigName] = pluginConfig[cfg.ConfigName]
		case CONFIG_LEVEL_CLIENT:
			npsPlugin.Configs[cfg.ConfigName] = clientConfig[cfg.ConfigName]
		case CONFIG_LEVEL_GLOBAL:
			npsPlugin.Configs[cfg.ConfigName] = globalConfig[cfg.ConfigName]
		}
	}
	return
}

// describe the stage of the plugin
func (npsPlugin *NpsPlugin) GetStage() []Stage {
	return []Stage{STAGE_RUN}
}

func (npsPlugin *NpsPlugin) Start(ctx context.Context) (context.Context, error) {
	return ctx, nil
}

func (npsPlugin *NpsPlugin) Run(ctx context.Context) (context.Context, error) {
	return ctx, nil
}

func (npsPlugin *NpsPlugin) End(ctx context.Context) (context.Context, error) {
	return ctx, nil
}

func (npsPlugin *NpsPlugin) GetClientConn(ctx context.Context) net.Conn {
	return ctx.Value(CLIENT_CONNECTION).(net.Conn)
}

func (npsPlugin *NpsPlugin) SetClientConn(ctx context.Context, conn net.Conn) context.Context {
	return context.WithValue(ctx, CLIENT_CONNECTION, conn)
}

func (npsPlugin *NpsPlugin) GetBridge(ctx context.Context) *bridge.Bridge {
	return ctx.Value(BRIDGE).(*bridge.Bridge)
}

func (npsPlugin *NpsPlugin) GetClientId(ctx context.Context) int {
	return ctx.Value(CLIENT_ID).(int)
}

type Plugins struct {
	StartPgs []Plugin
	RunPgs   []Plugin
	EndPgs   []Plugin
	AllPgs   []Plugin
}

func NewPlugins() *Plugins {
	p := &Plugins{}
	p.StartPgs = make([]Plugin, 0)
	p.RunPgs = make([]Plugin, 0)
	p.EndPgs = make([]Plugin, 0)
	p.AllPgs = make([]Plugin, 0)
	return p
}

func (pl *Plugins) Add(plugins ...Plugin) {
	for _, plugin := range plugins {
		for _, v := range plugin.GetStage() {
			pl.AllPgs = append(pl.RunPgs, plugin)
			switch v {
			case STAGE_RUN:
				pl.RunPgs = append(pl.RunPgs, plugin)
			case STAGE_END:
				pl.EndPgs = append(pl.EndPgs, plugin)
			case STAGE_START:
				pl.StartPgs = append(pl.StartPgs, plugin)
			}
		}
	}
}

func RunPlugin(ctx context.Context, pgs []Plugin, stage Stage) (context.Context, error) {
	var err error
	for _, pg := range pgs {
		switch stage {
		case STAGE_RUN:
			ctx, err = pg.Run(ctx)
		case STAGE_START:
			ctx, err = pg.Start(ctx)
		case STAGE_END:
			ctx, err = pg.End(ctx)
		}
		if err != nil {
			return ctx, err
		}
	}
	return ctx, nil
}
