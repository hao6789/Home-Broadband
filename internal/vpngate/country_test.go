package vpngate

import "testing"

func TestFlagOf(t *testing.T) {
	cases := map[string]string{
		"JP":   "🇯🇵",
		"jp":   "🇯🇵",
		"US":   "🇺🇸",
		"HK":   "🇭🇰",
		" kr ": "🇰🇷",
		"":     "",
		"J":    "",
		"JPN":  "",
		"J1":   "",
	}
	for in, want := range cases {
		if got := flagOf(in); got != want {
			t.Fatalf("flagOf(%q)=%q，想要 %q", in, got, want)
		}
	}
}

func TestCountryLabel(t *testing.T) {
	cases := []struct {
		code, fallback, want string
	}{
		{"JP", "Japan", "🇯🇵 日本"},
		{"jp", "Japan", "🇯🇵 日本"},
		{"KR", "Korea Republic of", "🇰🇷 韩国"},
		{"TW", "Taiwan", "🇹🇼 台湾"},
		// 中文表没收录时回退到清单给的英文名，但旗还在
		{"XK", "Kosovo", "🇽🇰 Kosovo"},
		// 连英文名也没有就只剩国家码
		{"XK", "", "🇽🇰 XK"},
		// 国家码不合法时只留名字，不能吐出半个旗
		{"", "Japan", "Japan"},
		{"JPN", "Japan", "Japan"},
	}
	for _, c := range cases {
		if got := CountryLabel(c.code, c.fallback); got != c.want {
			t.Fatalf("CountryLabel(%q,%q)=%q，想要 %q", c.code, c.fallback, got, c.want)
		}
	}
}

// 中文表里不该有空值或没配对的国家码。
func TestCountryTableSane(t *testing.T) {
	for code, name := range countryCN {
		if len(code) != 2 {
			t.Fatalf("国家码 %q 不是两位", code)
		}
		if flagOf(code) == "" {
			t.Fatalf("国家码 %q 出不了国旗", code)
		}
		if name == "" {
			t.Fatalf("%s 没有中文名", code)
		}
	}
}
