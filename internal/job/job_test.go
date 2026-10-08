package job

import (
	"sync"
	"testing"
)

// TestJobStoreConcurrent 并发创建/取消/关闭作业，race detector 下跑。
func TestJobStoreConcurrent(t *testing.T) {
	s := &JobStore{}
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			j := s.New("test", []string{"a", "b", "c"})
			// 并发 Set/Cancel/View/Dismiss
			j.Set(0, "running", "x")
			_ = j.Cancelled()
			_ = j.View()
			if i%2 == 0 {
				j.Cancel()
			}
			j.Finish()
			s.Dismiss(j.id)
		}(i)
	}
	wg.Wait()
	// 所有作业都应被 Dismiss 掉
	if n := len(s.Views()); n != 0 {
		t.Errorf("作业没清干净，剩 %d", n)
	}
}

// TestJobCancelIdempotent 取消多次不 panic。
func TestJobCancelIdempotent(t *testing.T) {
	s := &JobStore{}
	j := s.New("test", []string{"a"})
	j.Cancel()
	j.Cancel() // 第二次不应 panic
	j.Cancel()
	if !j.Cancelled() {
		t.Errorf("Cancel 后 Cancelled 应为 true")
	}
	j.Finish()
	if v := j.View(); v.Status != "cancelled" {
		t.Errorf("Finish 不应覆盖 cancelled，实际 %s", v.Status)
	}
}
