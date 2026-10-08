package web

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWriteServerError(t *testing.T) {
	// 验证 5xx 错误返回通用文案而非内部细节
	w := httptest.NewRecorder()

	internalErr := errors.New("dial tcp 10.0.0.1:443: connect: connection refused (internal path /etc/xray/config.json)")
	writeServerError(w, http.StatusBadGateway, internalErr, "上游服务异常，请稍后重试")

	resp := w.Result()
	if resp.StatusCode != http.StatusBadGateway {
		t.Errorf("状态码 = %d, want 502", resp.StatusCode)
	}

	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}
	if body["error"] != "上游服务异常，请稍后重试" {
		t.Errorf("error = %q, want 通用文案", body["error"])
	}
	// 确保内部细节没有外泄
	if strings.Contains(body["error"], "10.0.0.1") || strings.Contains(body["error"], "/etc/xray") {
		t.Errorf("内部细节外泄: %q", body["error"])
	}
}
