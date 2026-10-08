package config

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// TestStoreConcurrentRW 并发读写 config，race detector 下跑。
func TestStoreConcurrentRW(t *testing.T) {
	dir := t.TempDir()
	// 每个 goroutine 用独立 Store（per-dir 缓存），模拟多组件并发
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			s, err := Open(dir)
			if err != nil {
				t.Errorf("Open: %v", err)
				return
			}
			for j := 0; j < 20; j++ {
				_ = s.AddSession("tok", time.Now().Add(time.Hour))
				_ = s.ValidSession("tok")
				_ = s.GetWebSettings()
			}
		}(i)
	}
	wg.Wait()
	// 文件应完好可读
	if _, err := os.Stat(filepath.Join(dir, "config.json")); err != nil {
		t.Errorf("config.json 丢失: %v", err)
	}
}
