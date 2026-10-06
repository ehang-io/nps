package bridge

import (
	"fmt"
	"sync"
	"time"

	"ehang.io/nps/lib/file"

	"github.com/astaxie/beego/logs"
)

// An Alert is something on the bridge that an operator should look at: a
// connection that proved no key the server holds, or a vkey that turns up from
// two places at once.
type Alert struct {
	Time   time.Time `json:"time"`
	Kind   string    `json:"kind"`
	IP     string    `json:"ip"`
	Client int       `json:"client"` // 0 when no client is involved
	Detail string    `json:"detail"`
	Count  int       `json:"count"` // how many alike were folded into this one
}

const (
	alertKeep   = 200         // alerts kept, newest
	alertWindow = time.Minute // alike alerts within this are folded into the first
)

var alerts struct {
	sync.Mutex
	list []*Alert
	open map[string]*Alert // kind and ip -> the alert that is still absorbing repeats
}

// raise records an alert and logs it. A scanner or a wrong key that retries
// every few seconds is one alert with a count, not a flood.
func raise(kind, ip string, client int, detail string) {
	alerts.Lock()
	defer alerts.Unlock()
	if alerts.open == nil {
		alerts.open = make(map[string]*Alert)
	}
	k := kind + "|" + ip
	if a, ok := alerts.open[k]; ok && time.Since(a.Time) < alertWindow {
		a.Count++
		return
	}
	if len(alerts.open) >= alertKeep {
		// Sources that were quiet for the window have nothing left to fold
		// into. A spray from more addresses than that within one window just
		// loses its folding, never memory.
		for key, a := range alerts.open {
			if time.Since(a.Time) >= alertWindow {
				delete(alerts.open, key)
			}
		}
		if len(alerts.open) >= alertKeep {
			alerts.open = make(map[string]*Alert)
		}
	}
	a := &Alert{Time: time.Now(), Kind: kind, IP: ip, Client: client, Detail: detail, Count: 1}
	alerts.open[k] = a
	alerts.list = append(alerts.list, a)
	if len(alerts.list) > alertKeep {
		alerts.list = alerts.list[len(alerts.list)-alertKeep:]
	}
	logs.Warn("[ALERT] %s from %s: %s", kind, ip, detail)
}

// minVkeyLen is the shortest vkey that is not worth a warning. The proof a
// client sends lets whoever answers it guess a weak vkey offline, see
// bridgetls, so a short one is the one thing in the scheme that can go wrong
// by configuration. The keys nps makes are 16 random characters.
const minVkeyLen = 16

// warnWeakVkey raises an alert for a client whose vkey is short. The public
// vkey is public by design and not worth one.
func warnWeakVkey(id int, vkey, ip string) {
	if len(vkey) >= minVkeyLen {
		return
	}
	if c, err := file.GetDb().GetClient(id); err == nil && c.NoDisplay {
		return
	}
	raise("weak-vkey", ip, id, fmt.Sprintf("client %d has a vkey of %d characters: whoever answers as the server sees an HMAC of it and can guess it offline; use %d or more random characters", id, len(vkey), minVkeyLen))
}

// Alerts returns the alerts raised so far, newest first.
func Alerts() []Alert {
	alerts.Lock()
	defer alerts.Unlock()
	out := make([]Alert, 0, len(alerts.list))
	for i := len(alerts.list) - 1; i >= 0; i-- {
		out = append(out, *alerts.list[i])
	}
	return out
}
