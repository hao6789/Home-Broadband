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

func TestJobStoreCancel(t *testing.T) {
	s := &JobStore{}
	j := s.New("test", nil)
	id := j.View().ID

	// Store.Cancel 应设置 status 为 cancelled
	if !s.Cancel(id) {
		t.Fatalf("Cancel 运行中的作业应返回 true")
	}
	if v := j.View(); v.Status != "cancelled" {
		t.Errorf("Cancel 后状态 = %s, want cancelled", v.Status)
	}
	if !j.Cancelled() {
		t.Errorf("Cancel 后 Cancelled 应为 true")
	}

	// 已结束的作业 Cancel 应返回 false
	j2 := s.New("test2", nil)
	j2.Finish()
	if s.Cancel(j2.View().ID) {
		t.Errorf("Cancel 已结束的作业应返回 false")
	}

	// 不存在的 id 返回 false
	if s.Cancel("nonexistent") {
		t.Errorf("Cancel 不存在的 id 应返回 false")
	}
}
