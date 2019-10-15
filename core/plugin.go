package core

import (
	"context"
	"github.com/cnlh/nps/bridge"
	"net"
)

// Plugin interface, all plugins must implement those functions.
type Plugin interface {
	GetConfigName() *NpsConfigs
	GetConfigLevel() ConfigLevel
	GetStage() Stage
	Start(ctx context.Context, config map[string]string) (context.Context, error)
	Run(ctx context.Context, config map[string]string) (context.Context, error)
	End(ctx context.Context, config map[string]string) (context.Context, error)
}

type NpsPlugin struct {
	Version string
}

func (npsPlugin *NpsPlugin) GetConfigName() *NpsConfigs {
	return nil
}

// describe the config level
func (npsPlugin *NpsPlugin) GetConfigLevel() ConfigLevel {
	return CONFIG_LEVEL_PLUGIN
}

// describe the stage of the plugin
func (npsPlugin *NpsPlugin) GetStage() Stage {
	return STAGE_RUN
}

func (npsPlugin *NpsPlugin) Start(ctx context.Context, config map[string]string) (context.Context, error) {
	return ctx, nil
}

func (npsPlugin *NpsPlugin) Run(ctx context.Context, config map[string]string) (context.Context, error) {
	return ctx, nil
}

func (npsPlugin *NpsPlugin) End(ctx context.Context, config map[string]string) (context.Context, error) {
	return ctx, nil
}

func (npsPlugin *NpsPlugin) GetClientConn(ctx context.Context) net.Conn {
	return ctx.Value(CLIENT_CONNECTION).(net.Conn)
}

func (npsPlugin *NpsPlugin) GetBridge(ctx context.Context) *bridge.Bridge {
	return ctx.Value(BRIDGE).(*bridge.Bridge)
}

func (npsPlugin *NpsPlugin) GetClientId(ctx context.Context) int {
	return ctx.Value(CLIENT_ID).(int)
}

type Plugins struct {
	pgs []Plugin
}

func NewPlugins() *Plugins {
	p := &Plugins{}
	p.pgs = make([]Plugin, 0)
	return p
}

func (pl *Plugins) Add(plugins ...Plugin) {
	for _, plugin := range plugins {
		pl.pgs = append(pl.pgs, plugin)
	}
}
