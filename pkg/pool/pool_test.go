package pool

import (
	"testing"
	"time"
)

// TestPoolStartStop 测试工作池的启动和停止功能
func TestPoolStartStop(t *testing.T) {
	p := newPool(2, 5, 100, 1*time.Second, 1*time.Second)
	if p == nil {
		t.Errorf("Expected a non-nil pool")
	}

	p.startWorker()
}
