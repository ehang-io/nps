package pmux

import (
	"testing"
)

func TestPortMuxStartAndClose(t *testing.T) {
	// port 0 lets the kernel pick a free port, the old test hard coded 8888
	// and failed whenever that port was taken by another process
	pMux, err := NewPortMux(0, "Ds")
	if err != nil {
		t.Fatalf("new port mux: %v", err)
	}
	if pMux.Listener == nil {
		t.Fatal("listener is nil after start")
	}
	for name, l := range map[string]interface{ Close() error }{
		"client":  pMux.GetClientListener(),
		"http":    pMux.GetHttpListener(),
		"https":   pMux.GetHttpsListener(),
		"manager": pMux.GetManagerListener(),
	} {
		if l == nil {
			t.Fatalf("%s listener is nil", name)
		}
	}
	if err := pMux.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	// closing twice must not panic on double channel close
	if err := pMux.Close(); err != nil {
		t.Logf("second close returned: %v", err)
	}
}

func TestPortMuxInvalidPort(t *testing.T) {
	if _, err := NewPortMux(-1, "Ds"); err == nil {
		t.Fatal("expected an error for an invalid port")
	}
}
