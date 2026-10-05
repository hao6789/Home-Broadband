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
	_, _ = w.Write([]byte(indexHTML))
}
