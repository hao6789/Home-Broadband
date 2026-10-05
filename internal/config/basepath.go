package config

import (
	"crypto/rand"
	"fmt"
	"net/http"
	"strings"
)

// basePathAlphabet 避开容易看错的字符。
const basePathAlphabet = "abcdefghijkmnpqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789"

// InitBasePath 载入访问路径（没有就生成一个）。返回是否本次新建。
func InitBasePath(dir string) (bool, error) {
	s, err := Open(dir)
	if err != nil {
		return false, err
	}
	setGlobal(s)
	_, created, err := s.LoadBasePath()
	return created, err
}

// LoadBasePath 读取或生成随机访问路径，形如 /aB3xY9pQ。
// 和 3x-ui 一样：路径本身也是一层门槛，扫端口的探不到界面。
func LoadBasePath(dir string) (string, bool, error) {
	s, err := Open(dir)
	if err != nil {
		return "", false, err
	}
	return s.LoadBasePath()
}

// CurrentBasePath 返回当前访问路径（形如 /xxx 或空）。
func CurrentBasePath() string {
	if s := globalStore(); s != nil {
		return s.CurrentBasePath()
	}
	return ""
}

// SetBasePath 校验并保存新的访问路径，立即生效。空串表示不加路径前缀。
func SetBasePath(raw string) (string, error) {
	if s := globalStore(); s != nil {
		return s.SetBasePath(raw)
	}
	return "", fmt.Errorf("配置存储未初始化")
}

func randomBasePath(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	out := make([]byte, n)
	for i, v := range b {
		out[i] = basePathAlphabet[int(v)%len(basePathAlphabet)]
	}
	return string(out), nil
}

// normalizeBasePath 统一成 /xxx 的形式（无结尾斜杠）。
func normalizeBasePath(bp string) string {
	bp = strings.Trim(bp, "/")
	if bp == "" {
		return ""
	}
	return "/" + bp
}

// StripBasePath 把请求剥掉前缀后交给内层 handler。
// 前缀不匹配的请求一律 404，不泄漏这里跑着什么服务。
// 每次请求读当前 basePath，改路径后无需重启即可生效。
func StripBasePath(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		base := CurrentBasePath()
		if base == "" {
			next.ServeHTTP(w, r)
			return
		}
		switch {
		case r.URL.Path == base:
			// 少了结尾斜杠时补上，否则页面里的相对路径会拼错
			http.Redirect(w, r, base+"/", http.StatusTemporaryRedirect)
		case strings.HasPrefix(r.URL.Path, base+"/"):
			r.URL.Path = strings.TrimPrefix(r.URL.Path, base)
			next.ServeHTTP(w, r)
		default:
			http.NotFound(w, r)
		}
	})
}
