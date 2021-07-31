package planner

import (
	"container/heap"
)

func DijkstraFindPath(start interface{}, coster NodeCoster, isGoal NodeIsGoaler, nGen NeighborGenerator) (map[interface{}]interface{}, map[interface{}]float64, interface{}) {
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

		if isGoal(current) {
			final = current
			break
		}

		//fmt.Printf("current: %s\n", current)

		neighbors := nGen(current)
		for _, node := range neighbors {
			newCost := costSoFar[current] + coster(current, node)
			existingNeighborCost, found := costSoFar[node]
			if !found || newCost < existingNeighborCost {
				costSoFar[node] = newCost
				newNeighbor := &Neighbor{
					value: node,
					cost:  newCost,
				}
				//fmt.Printf("added to frontier: %s\n", node)
				heap.Push(frontier, newNeighbor)
				cameFrom[node] = current
			}
		}
	}

	return cameFrom, costSoFar, final
}
