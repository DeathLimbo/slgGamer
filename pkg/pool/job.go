package pool

import "context"

type Job struct {
	priority int // larger = higher priority
	fn       JobFunc
	ctx      context.Context
	resultCh chan Result
	index    int // heap index
	// internal: sequence to maintain FIFO among equal priorities (optional)
	seq uint64
}

// ---------- Job / Result / Future ----------
type JobFunc func(ctx context.Context) (interface{}, error)

// ---------- Priority Queue (heap) ----------
type jobPQ []*Job

func (pq jobPQ) Len() int { return len(pq) }
func (pq jobPQ) Less(i, j int) bool {
	// higher priority first; if equal, lower seq (earlier) first
	if pq[i].priority == pq[j].priority {
		return pq[i].seq < pq[j].seq
	}
	return pq[i].priority > pq[j].priority
}
func (pq jobPQ) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
	pq[i].index = i
	pq[j].index = j
}
func (pq *jobPQ) Push(x interface{}) {
	n := len(*pq)
	item := x.(*Job)
	item.index = n
	*pq = append(*pq, item)
}

func (pq *jobPQ) Pop() interface{} {
	old := *pq
	n := len(old)
	if n == 0 {
		return nil
	}
	item := old[n-1]
	item.index = -1
	*pq = old[0 : n-1]
	return item
}
