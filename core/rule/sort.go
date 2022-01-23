package rule

import "ehang.io/nps/core/process"

type Sort []*Rule

func (s Sort) Len() int      { return len(s) }
func (s Sort) Swap(i, j int) { s[i], s[j] = s[j], s[i] }
// Less rule sort by
func (s Sort) Less(i, j int) bool {
	iHandlerSort := orderMap[s[i].Handler.GetName()]
	iProcessSort := orderMap[s[i].Process.GetName()]
	jHandlerSort := orderMap[s[j].Handler.GetName()]
	jProcessSort := orderMap[s[j].Process.GetName()]
	iSort := iHandlerSort<<16 | iProcessSort<<8
	jSort := jHandlerSort<<16 | jProcessSort<<8
	if vi, ok := s[i].Process.(*process.HttpServeProcess); ok {
		if vj, ok := s[j].Process.(*process.HttpServeProcess); ok {
			iSort = iSort | (len(vj.RouteUrl) & (2 ^ 8 - 1))
			jSort = jSort | (len(vi.RouteUrl) & (2 ^ 8 - 1))
		}
	}
	if vi, ok := s[i].Process.(*process.HttpsServeProcess); ok {
		if vj, ok := s[j].Process.(*process.HttpsServeProcess); ok {
			iSort = iSort | (len(vj.RouteUrl) & (2 ^ 8 - 1))
			jSort = jSort | (len(vi.RouteUrl) & (2 ^ 8 - 1))
		}
	}
	return iSort > jSort
}
