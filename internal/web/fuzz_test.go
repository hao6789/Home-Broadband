package web

import (
	"testing"
)

// FuzzParseSemver 确保各种畸形版本字符串不 panic、不死循环。
func FuzzParseSemver(f *testing.F) {
	seeds := []string{"v1.2.3", "1.2.3", "v0.1.26", "", "dev", "v1", "v1.2", "v1.2.3.4", "abc", "v999.999.999"}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, v string) {
		// 不应 panic
		ver, ok := parseSemver(v)
		// ok=true 时数值必须在合理范围
		if ok {
			for _, n := range ver {
				if n < 0 {
					t.Errorf("parseSemver(%q) 返回负数 %v", v, ver)
				}
			}
		}
		// versionLess 也不应 panic
		_ = versionLess(v, "v1.0.0")
		_ = versionLess("v1.0.0", v)
	})
}
