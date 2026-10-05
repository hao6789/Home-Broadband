package config

import (
	"os"
	"testing"
)

func TestNormalizeListenAddr(t *testing.T) {
	cases := map[string]string{
		"":          "",
		"0.0.0.0":   "",
		"all":       "",
		"127.0.0.1": "127.0.0.1",
	}
	for in, want := range cases {
		got, err := NormalizeListenAddr(in)
		if err != nil {
			t.Fatalf("NormalizeListenAddr(%q) 意外报错: %v", in, err)
		}
		if got != want {
			t.Fatalf("NormalizeListenAddr(%q)=%q，想要 %q", in, got, want)
		}
	}
	if _, err := NormalizeListenAddr("not-an-ip"); err == nil {
		t.Fatal("非法监听地址应当报错")
	}
}

func TestValidatePort(t *testing.T) {
	for _, p := range []int{1, 8899, 65535} {
		if err := ValidatePort(p); err != nil {
			t.Fatalf("端口 %d 应合法: %v", p, err)
		}
	}
	for _, p := range []int{0, -1, 70000} {
		if err := ValidatePort(p); err == nil {
			t.Fatalf("端口 %d 应非法", p)
		}
	}
}

func TestSetBasePathValidatesAndPersists(t *testing.T) {
	dir := t.TempDir()
	if _, err := InitBasePath(dir); err != nil {
		t.Fatalf("InitBasePath: %v", err)
	}
	bp, err := SetBasePath("myPanel_1")
	if err != nil {
		t.Fatalf("SetBasePath: %v", err)
	}
	if bp != "/myPanel_1" || CurrentBasePath() != "/myPanel_1" {
		t.Fatalf("basePath 未生效: %q / %q", bp, CurrentBasePath())
	}
	if _, err := os.ReadFile(dir + "/basepath"); err != nil {
		t.Fatalf("basepath 未落盘: %v", err)
	}
	if _, err := SetBasePath("bad/slash"); err == nil {
		t.Fatal("带非法字符的路径应被拒")
	}
	// 空串表示去掉前缀
	if bp, err := SetBasePath(""); err != nil || bp != "" {
		t.Fatalf("空路径应清空前缀: %q %v", bp, err)
	}
}

// 界面上改过端口会落盘。之后再带 -web 启动时，命令行必须说话算话，
// 否则用户敲了参数却连不上，还没有任何提示（ct-54 上真实踩到）。
func TestLoadWebSettingsExplicitFlagWins(t *testing.T) {
	dir := t.TempDir()

	// 首次启动：建档存 8899
	if _, err := LoadWebSettings(dir, 8899, false); err != nil {
		t.Fatalf("首次: %v", err)
	}

	// 不带 -web 重启：沿用盘上的 8899，不被默认值覆盖
	s, err := LoadWebSettings(dir, 8899, false)
	if err != nil {
		t.Fatalf("沿用: %v", err)
	}
	if s.Port != 8899 {
		t.Fatalf("没显式指定时应沿用盘上的值，实际 %d", s.Port)
	}

	// 显式 -web 80：以命令行为准
	s, err = LoadWebSettings(dir, 80, true)
	if err != nil {
		t.Fatalf("显式指定: %v", err)
	}
	if s.Port != 80 {
		t.Fatalf("显式 -web 应压过盘上的值，实际 %d", s.Port)
	}

	// 且要落盘，下次不带参数启动仍是 80
	s, err = LoadWebSettings(dir, 8899, false)
	if err != nil {
		t.Fatalf("复读: %v", err)
	}
	if s.Port != 80 {
		t.Fatalf("显式指定的端口应已写回，实际 %d", s.Port)
	}
}
