package planner

import "container/heap"

// A Neighbor is something we manage in a neighbor-priority queue.
type Neighbor struct {
	value interface{} // The value of the item; arbitrary.
	cost  float64     // The cost of the item in the queue.
	// The index is needed by update and is maintained by the heap.Interface methods.
	index int // The index of the item in the heap.
}

// A NeighborQueue implements heap.Interface and holds Neighbors.
type NeighborQueue []*Neighbor

func (nq NeighborQueue) Len() int { return len(nq) }

func (nq NeighborQueue) Less(i, j int) bool {
	// We want Pop to give us the lowest, not highest, cost so we use lesser than here.
	return nq[i].cost < nq[j].cost
}

func (nq NeighborQueue) Swap(i, j int) {
	nq[i], nq[j] = nq[j], nq[i]
	nq[i].index = i
	nq[j].index = j
}

func (nq *NeighborQueue) Push(x interface{}) {
	n := len(*nq)
	item := x.(*Neighbor)
	item.index = n
	*nq = append(*nq, item)
}

func (nq *NeighborQueue) PushWrapper(n *Neighbor) {
	nq.Push(n)
}

func (nq *NeighborQueue) Pop() interface{} {
	old := *nq
	n := len(old)
	item := old[n-1]
	item.index = -1 // for safety
	*nq = old[0 : n-1]
	return item
}

func (nq *NeighborQueue) PopWrapper() *Neighbor {
	x := nq.Pop()
	return x.(*Neighbor)
}

// update modifies the cost and value of an Neighbor in the queue.
func (nq *NeighborQueue) update(item *Neighbor, cost float64) {
	item.cost = cost
	heap.Fix(nq, item.index)
}
