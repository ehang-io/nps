package common

import "time"

const (
	CONN_DATA_SEQ     = "*#*" //Separator
	VERIFY_EER        = "vkey"
	VERIFY_SUCCESS    = "sucs"
	VERIFY_PROTOCOL   = "prot" // the client speaks another protocol revision
	WORK_MAIN         = "main"
	WORK_CHAN         = "chan"
	WORK_CONFIG       = "conf"
	WORK_REGISTER     = "rgst"
	WORK_SECRET       = "sert"
	WORK_SECRET_MUX   = "smux" // a visitor's long-lived link, a mux of secret streams
	WORK_FILE         = "file"
	WORK_P2P          = "p2pm"
	WORK_P2P_VISITOR  = "p2pv"
	WORK_P2P_PROVIDER = "p2pp"
	WORK_P2P_CONNECT  = "p2pc"
	WORK_P2P_SUCCESS  = "p2ps"
	WORK_P2P_END      = "p2pe"
	WORK_P2P_LAST     = "p2pl"
	WORK_STATUS       = "stus"
	WORK_UPDATE       = "updt" //server asks a client to update itself
	RES_MSG           = "msg0"
	RES_CLOSE         = "clse"
	NEW_UDP_CONN      = "udpc" //p2p udp conn
	NEW_TASK          = "task"
	NEW_CONF          = "conf"
	NEW_HOST          = "host"
	CONN_TCP          = "tcp"
	CONN_UDP          = "udp"
	UnauthorizedBytes = `HTTP/1.1 401 Unauthorized
Content-Type: text/plain; charset=utf-8
WWW-Authenticate: Basic realm="easyProxy"

401 Unauthorized`
	ConnectionFailBytes = `HTTP/1.1 404 Not Found

`
)

// A secret link carries visitors' connections, so a link that has silently
// died must be given up in seconds, not in the minutes that suit the control
// channel: whatever is sent into it waits as long. It is pinged every
// SecretLinkPingEvery and given up after SecretLinkPingCheck pings go
// unanswered, and a link with two unanswered is not given new streams.
const (
	SecretLinkPingEvery = time.Second
	SecretLinkPingCheck = 5
)
