// Copyright 2014 beego Author. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package logs

import (
	"bytes"
	"strings"
	"encoding/json"
	"fmt"
	"time"
)

const (
	apacheFormatPattern = "%s - - [%s] \"%s %d %d\" %f %s %s"
	apacheFormat        = "APACHE_FORMAT"
	jsonFormat          = "JSON_FORMAT"
)

// AccessLogRecord struct for holding access log data.
type AccessLogRecord struct {
	RemoteAddr     string        `json:"remote_addr"`
	RequestTime    time.Time     `json:"request_time"`
	RequestMethod  string        `json:"request_method"`
	Request        string        `json:"request"`
	ServerProtocol string        `json:"server_protocol"`
	Host           string        `json:"host"`
	Status         int           `json:"status"`
	BodyBytesSent  int64         `json:"body_bytes_sent"`
	ElapsedTime    time.Duration `json:"elapsed_time"`
	HTTPReferrer   string        `json:"http_referrer"`
	HTTPUserAgent  string        `json:"http_user_agent"`
	RemoteUser     string        `json:"remote_user"`
}

func (r *AccessLogRecord) json() ([]byte, error) {
	buffer := &bytes.Buffer{}
	encoder := json.NewEncoder(buffer)
	disableEscapeHTML(encoder)

	err := encoder.Encode(r)
	return buffer.Bytes(), err
}

func disableEscapeHTML(i interface{}) {
	if e, ok := i.(interface {
		SetEscapeHTML(bool)
	}); ok {
		e.SetEscapeHTML(false)
	}
}

// AccessLog - Format and print access log.
func AccessLog(r *AccessLogRecord, format string) {
	var msg string
	switch format {
	case apacheFormat:
		timeFormatted := r.RequestTime.Format("02/Jan/2006 03:04:05")
		msg = fmt.Sprintf(apacheFormatPattern, r.RemoteAddr, timeFormatted, r.Request, r.Status, r.BodyBytesSent,
			r.ElapsedTime.Seconds(), r.HTTPReferrer, r.HTTPUserAgent)
	case jsonFormat:
		fallthrough
	default:
		jsonData, err := r.json()
		if err != nil {
			msg = fmt.Sprintf(`{"Error": "%s"}`, err)
		} else {
			msg = string(jsonData)
		}
	}
	beeLogger.writeMsg(levelLoggerImpl, strings.TrimSpace(msg))
}
