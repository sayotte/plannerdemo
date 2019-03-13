package planner

import (
	"container/heap"
)

type NodeCoster func(src, dst interface{}) float64
type NodeIsGoaler func(n interface{}) bool
type NodeEstimator func(dst interface{}) float64
type NeighborGenerator func(n interface{}) []interface{}

func AStarFindPath(start interface{}, coster NodeCoster, estimator NodeEstimator, isGoaler NodeIsGoaler, nGen NeighborGenerator) (map[interface{}]interface{}, map[interface{}]float64, interface{}) {
	startNode := &Neighbor{
		value: start,
		cost:  0.0,
	}

	frontier := &NeighborQueue{}
	heap.Push(frontier, startNode)

	cameFrom := make(map[interface{}]interface{})
	costSoFar := make(map[interface{}]float64)
	cameFrom[start] = nil
	costSoFar[start] = 0

	var final interface{}
	for frontier.Len() > 0 {
		currentI := heap.Pop(frontier)
		current := currentI.(*Neighbor).value

		if isGoaler(current) {
			final = current
			break
		}

		//fmt.Printf("current: %s\n", current)

		for _, node := range nGen(current) {
			newCost := costSoFar[current] + coster(current, node)
			existingNeighborCost, found := costSoFar[node]
			if !found || newCost < existingNeighborCost {
				costSoFar[node] = newCost
				estimatedCost := estimator(node)
				priority := newCost + estimatedCost
				newNeighbor := &Neighbor{
					value: node,
					cost:  priority,
				}
				//fmt.Printf(
				//	"Adding neighbor %q with cost %f, estimate %f, priority %.3f\n",
				//	node,
				//	newCost,
				//	estimatedCost,
				//	priority,
				//)
				heap.Push(frontier, newNeighbor)
				cameFrom[node] = current
			}
		}
	}

	return cameFrom, costSoFar, final
}
