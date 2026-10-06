package controllers

import (
	"sort"
	"time"

	"ehang.io/nps/bridge"
	"ehang.io/nps/lib/file"
	"ehang.io/nps/lib/sysstat"
	"ehang.io/nps/server"
)

// StatsController shows the host stats clients report on their mux pings.
type StatsController struct {
	BaseController
}

// maxChartPoints caps the points History returns, so two hours of a client's
// records are thinned for the chart.
const maxChartPoints = 240

// the stats of every client, so only an admin sees them
func (s *StatsController) adminOnly() {
	if s.Data["isAdmin"] != true {
		s.Ctx.Output.SetStatus(403)
		s.StopRun()
	}
}

func (s *StatsController) Index() {
	s.adminOnly()
	s.Data["menu"] = "stats"
	s.SetInfo("host stats")
	s.display("stats/index")
}

type gpuView struct {
	Power float64 `json:"power"`
	Limit float64 `json:"limit"`
}

type statsRow struct {
	Id      int       `json:"id"`
	Remark  string    `json:"remark"`
	Online  bool      `json:"online"`
	Age     int       `json:"age"` // seconds since the last record, -1 if none
	CPU     float64   `json:"cpu"`
	Mem     float64   `json:"mem"`
	GPUCnt  int       `json:"gpu_count"`
	GPUs    []gpuView `json:"gpus"`
	HasData bool      `json:"has_data"`
}

// Latest answers with the newest record of every client.
func (s *StatsController) Latest() {
	s.adminOnly()
	rows := make([]statsRow, 0)
	now := time.Now()
	file.GetDb().JsonDb.Clients.Range(func(_, value interface{}) bool {
		c := value.(*file.Client)
		if c.NoDisplay {
			return true
		}
		r := statsRow{Id: c.Id, Remark: c.Remark, Age: -1, GPUs: []gpuView{}}
		_, r.Online = server.Bridge.Client.Load(c.Id)
		if p, ok := server.Bridge.ClientLastStats(c.Id); ok {
			if st, ok := sysstat.Decode(p.Rec[:]); ok {
				r.HasData = true
				r.Age = int(now.Sub(p.At).Seconds())
				r.CPU, r.Mem, r.GPUCnt = st.CPU, st.Mem, st.GPUs
				for i := range st.Power {
					r.GPUs = append(r.GPUs, gpuView{st.Power[i], st.Limit[i]})
				}
			}
		}
		rows = append(rows, r)
		return true
	})
	sort.Slice(rows, func(i, j int) bool { return rows[i].Id < rows[j].Id })
	s.Data["json"] = map[string]interface{}{"rows": rows, "total": len(rows)}
	s.ServeJSON()
	s.StopRun()
}

// Alerts answers with what the bridge has flagged, newest first: connections
// that proved no key the server holds, and keys that appear from two places.
// Repeats within a minute are folded into one entry with a count.
func (s *StatsController) Alerts() {
	s.adminOnly()
	rows := bridge.Alerts()
	s.Data["json"] = map[string]interface{}{"rows": rows, "total": len(rows)}
	s.ServeJSON()
	s.StopRun()
}

type histPoint struct {
	T     int64     `json:"t"` // unix ms
	CPU   float64   `json:"cpu"`
	Mem   float64   `json:"mem"`
	Power []float64 `json:"power"`
}

// History answers with a client's records of the last two hours, thinned to
// at most maxChartPoints.
func (s *StatsController) History() {
	s.adminOnly()
	pts := server.Bridge.ClientStats(s.GetIntNoErr("id"))
	step := (len(pts) + maxChartPoints - 1) / maxChartPoints
	if step < 1 {
		step = 1
	}
	out := make([]histPoint, 0, len(pts)/step+1)
	// from the newest point back, so the chart's right edge is the latest
	// record whatever the step; out is then reversed to oldest first
	for i := len(pts) - 1; i >= 0; i -= step {
		st, ok := sysstat.Decode(pts[i].Rec[:])
		if !ok {
			continue
		}
		if st.Power == nil {
			st.Power = []float64{}
		}
		out = append(out, histPoint{pts[i].At.UnixMilli(), st.CPU, st.Mem, st.Power})
	}
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	s.Data["json"] = map[string]interface{}{"rows": out}
	s.ServeJSON()
	s.StopRun()
}
