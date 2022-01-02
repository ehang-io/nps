package common

const (
	CONN_DATA_SEQ     = "*#*" //Separator
	VERIFY_EER        = "vkey"
	VERIFY_SUCCESS    = "sucs"
	WORK_MAIN         = "main"
	WORK_CHAN         = "chan"
	WORK_CONFIG       = "conf"
	WORK_REGISTER     = "rgst"
	WORK_SECRET       = "sert"
	WORK_FILE         = "file"
	WORK_P2P          = "p2pm"
	WORK_P2P_VISITOR  = "p2pv"
	WORK_P2P_PROVIDER = "p2pp"
	WORK_P2P_CONNECT  = "p2pc"
	WORK_P2P_SUCCESS  = "p2ps"
	WORK_P2P_END      = "p2pe"
	WORK_P2P_LAST     = "p2pl"
	WORK_STATUS       = "stus"
	RES_MSG           = "msg0"
	RES_CLOSE         = "clse"
	NEW_UDP_CONN      = "udpc" //p2p udp conn
	NEW_TASK          = "task"
	NEW_CONF          = "conf"
	NEW_HOST          = "host"
	CONN_TCP          = "tcp"
	CONN_UDP          = "udp"
	CONN_TEST         = "TST"
	UnauthorizedBytes = `HTTP/1.1 407 Proxy Authentication Required
Server: Proxy
Proxy-Authenticate: Basic realm="easyProxy Authentication"
Connection: Close
Proxy-Connection: Close
Content-Length: 0

`

	ConnectionFailBytes = `HTTP/1.1 404 Not Found

`
)
