package tunnel

import (
	"encoding/json"
	"strings"
	"testing"

	"home-broadband/internal/vpngate"
)

// MarshalJSON 必须输出与原来一致的形状，且不能与后台写竞争。
func TestTunnelMarshalJSONShape(t *testing.T) {
	tn := &Tunnel{Slot: 1, Port: 40000, Node: vpngate.Node{HostName: "h"}}
	tn.setStatus("up")
	tn.setExitIP("1.2.3.4")
	tn.setErr("")
	b, err := json.Marshal(tn)
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, want := range []string{`"slot":1`, `"port":40000`, `"status":"up"`, `"exit_ip":"1.2.3.4"`} {
		if !strings.Contains(s, want) {
			t.Fatalf("缺 %s，实际 %s", want, s)
		}
	}
	// 未导出字段不能漏出去
	if strings.Contains(s, "listener") || strings.Contains(s, "swapped") {
		t.Fatalf("内部字段泄漏: %s", s)
	}
}
