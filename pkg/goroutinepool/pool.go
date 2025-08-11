package goroutinepool

import "time"

var goPool pool

type pool interface {
	Num() int32
	Run()
	AddTask(t Task)
	StartGo()
}

type gor interface {
}

// 如何保证协程安全呢
type basePool struct {
	num   int32
	tasks chan Task
}

func (b *basePool) Num() int32 {
	return b.num
}

func (b *basePool) AddTask(task Task) {
	t := time.After(100 * time.Millisecond) // 100 ms
	select {
	case <-t: // 塞不进去，应当新启协程，保证能立马执行
	case b.tasks <- task:

	}
}

func (b *basePool) StartGo() {

}

func (b *basePool) Run() {
	select {
	case t := <-b.tasks: //任务来了该怎么办

	}
}

func NewPool() {
	goPool = &basePool{}
	goPool.Run()
}

/*
该函数应当保证执行的f都可使用执行，而不是返回错误不管了
*/
func Go(f func()) {
	t := NewTask(f)
	defer func() {
	}()
}
