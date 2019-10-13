package core

import (
	"context"
)

// This structure is used to describe the plugin configuration item name and description.
type Config struct {
	ConfigName  string
	Description string
}

type Stage uint8

// These constants are meant to describe the stage in which the plugin is running.
const (
	STAGE_START_RUN_END Stage = iota
	STAGE_START_RUN
	STAGE_START_END
	STAGE_RUN_END
	STAGE_START
	STAGE_END
	STAGE_RUN
)

// Plugin interface, all plugins must implement those functions.
type Plugin interface {
	GetConfigName() []*Config
	GetBeforePlugin() Plugin
	GetStage() Stage
	Start(ctx context.Context, config map[string]string) error
	Run(ctx context.Context, config map[string]string) error
	End(ctx context.Context, config map[string]string) error
}
