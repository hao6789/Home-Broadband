package vpngate

import (
	"testing"
)

func TestIsResidential(t *testing.T) {
	cases := []struct {
		host, ip string
		want     bool
	}{
		// vpngate 自营：hostname 前缀
		{"public-vpn-123", "1.2.3.4", false},
		{"PUBLIC-VPN-9", "1.2.3.4", false},
		// vpngate 自营：学术网络机房那一段
		{"vpn829571948", "219.100.37.117", false},
		{"vpn123", "219.100.37.0", false},
		{"vpn123", "219.100.37.255", false},
		// 志愿者家宽
		{"vpn829571948", "60.101.169.14", true},
		{"vpn123", "219.100.38.1", true},
		{"vpn123", "219.100.36.255", true},
		// IP 读不出来时不敢断言，放过
		{"vpn123", "", true},
		{"vpn123", "不是IP", true},
	}
	for _, c := range cases {
		if got := isResidential(c.host, c.ip); got != c.want {
			t.Fatalf("isResidential(%q,%q)=%v，想要 %v", c.host, c.ip, got, c.want)
		}
	}
}

// 解析 CSV 时就把家宽标记算好，别留给调用方各自判断。
func TestParseNodeCSVMarksResidential(t *testing.T) {
	nodes, err := parseNodeCSV(sampleNodeCSV("public-vpn-200"))
	if err != nil {
		t.Fatal(err)
	}
	if nodes[0].Residential {
		t.Fatal("public-vpn- 开头的是自营机房，不该标成家宽")
	}
	nodes, err = parseNodeCSV(sampleNodeCSV("vpn829571948"))
	if err != nil {
		t.Fatal(err)
	}
	if !nodes[0].Residential {
		t.Fatal("志愿者节点应标成家宽")
	}
}
