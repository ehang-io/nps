package core

import (
	"errors"
)

type Stage uint8

// These constants are meant to describe the stage in which the plugin is running.
const (
	STAGE_START Stage = iota
	STAGE_END
	STAGE_RUN
	PROXY_CONNECTION_TYPE = "proxy_target_type"
	PROXY_CONNECTION_ADDR = "proxy_target_addr"
	PROXY_CONNECTION_PORT = "proxy_target_port"
	CLIENT_CONNECTION     = "clientConn"
	BRIDGE                = "bridge"
	CLIENT_ID             = "client_id"
)

type ConfigLevel uint8

const (
	CONFIG_LEVEL_CLIENT ConfigLevel = iota
	CONFIG_LEVEL_PLUGIN
	CONFIG_LEVEL_GLOBAL
)

var (
	CLIENT_CONNECTION_NOT_EXIST = errors.New("the client connection is not exist")
	BRIDGE_NOT_EXIST            = errors.New("the bridge is not exist")
	REQUEST_EOF                 = errors.New("the request has finished")
	CLIENT_ID_NOT_EXIST         = errors.New("the client id is not exist")
)
