package goroutinepool

type Task interface {
	GetTask() func()
}

type baseTask struct {
	f func()
}

func (b *baseTask) GetTask() func() {
	return b.f
}
func NewTask(f func()) Task {
	ret := &baseTask{
		f: f,
	}
	return ret
}
