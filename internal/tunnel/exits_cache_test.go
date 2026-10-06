package tunnel

import (
	"testing"
	"time"
)

// 验证缓存命中时 BoundUp 会用最新的 live 重算
func TestCachedInboundsRefreshesBoundUp(t *testing.T) {
	// 先污染缓存：live 为空
	InvalidateInbounds()
	ibCache.mu.Lock()
	ibCache.at = time.Now()
	ibCache.list = []Inbound{
		{ID: 1, Tag: "tag1", BoundTo: "host-a", BoundUp: false},
	}
	ibCache.err = nil
	ibCache.mu.Unlock()

	// 用新的 live 查询，host-a 现在在线了
	list, err := CachedInbounds(nil, map[string]bool{"host-a": true})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("want 1 inbound, got %d", len(list))
	}
	if !list[0].BoundUp {
		t.Fatalf("BoundUp 应该被最新的 live 重算为 true")
	}

	// 确认缓存本身没被污染（下次用空 live 查应该还是 false）
	InvalidateInbounds()
	ibCache.mu.Lock()
	ibCache.at = time.Now()
	ibCache.list = []Inbound{
		{ID: 1, Tag: "tag1", BoundTo: "host-a", BoundUp: false},
	}
	ibCache.err = nil
	ibCache.mu.Unlock()

	list2, _ := CachedInbounds(nil, map[string]bool{})
	if list2[0].BoundUp {
		t.Fatalf("空 live 下 BoundUp 应该是 false")
	}
}
