package handler

import "ehang.io/nps/lib/enet"

type RedisHandler struct {
	DefaultHandler
}

func (rds *RedisHandler) GetName() string {
	return "redis"
}

func (rds *RedisHandler) GetZhName() string {
	return "redis协议"
}

func (rds *RedisHandler) HandleConn(b []byte, c enet.Conn) (bool, error) {
	if b[0] == 42 && b[1] == 49 && b[2] == 13 {
		return rds.processConn(c)
	}
	return false, nil
}
