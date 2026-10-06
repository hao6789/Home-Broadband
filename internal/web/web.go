package web

import (
	_ "embed"
	"net/http"
)

// Version 由 main 在启动时从构建注入的 version 同步过来。
var Version = "dev"

//go:embed static/index.html
var indexHTML string

func handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	// 前端是 go:embed 打进二进制里的，发版即变：禁止浏览器缓存，
	// 否则用户更新二进制后还看到旧版页面（如 v0.1.14 的地区列表 bug）
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	_, _ = w.Write([]byte(indexHTML))
}
