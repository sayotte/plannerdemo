package planner

import (
	"fmt"
	"gopkg.in/yaml.v2"
	"log"
	"math"
	"sort"
	"strings"
	"time"
)

type MaintenancePlanner struct {
	expansions int
}

func (mp *MaintenancePlanner) PlanActionsForTargetRevision(startingState State, targetSoftwareRevision int) []MaintenanceAction {
	coster := func(src, dst interface{}) float64 {
		return 1.0
	}

	isGoaler := func(n interface{}) bool {
		action := n.(MaintenanceAction)
		state := action.FinalState()
		for _, nodeState := range state {
			if nodeState.SoftwareRevision != targetSoftwareRevision {
				return false
			}
			if !nodeState.InLoadbalancerPool {
				return false
			}
		}
		return true
	}

	estimator := func(n interface{}) float64 {
		action := n.(MaintenanceAction)
		return estimateAction(action, targetSoftwareRevision)
		//startingState := n.(MaintenanceAction).FinalState()
		//var cost float64
		//for _,node := range startingState {
		//	cost += baseEstimateForNode(node, startingState, targetSoftwareRevision)
		//}
		//return cost
	}

	availableActionPrototypes := []MaintenanceAction{
		&DrainNodeFromPoolAction{TargetRevision: targetSoftwareRevision},
		&StopAppAction{TargetRevision: targetSoftwareRevision},
		&UpdateSoftwareRevisionAction{TargetRevision: targetSoftwareRevision},
		&StartAppAction{TargetRevision: targetSoftwareRevision},
		&WarmCacheAction{TargetRevision: targetSoftwareRevision},
		&AddNodeToPoolAction{TargetRevision: targetSoftwareRevision},
	}
	neighborGen := func(n interface{}) []interface{} {
		mp.expansions += 1

		startingState := n.(MaintenanceAction).FinalState()
		var possibleActions []MaintenanceAction
		for _, actionProto := range availableActionPrototypes {
			possibleActions = append(possibleActions, actionProto.CloneForValidTargets(startingState)...)
		}
		// have to return []interface{}
		ret := make([]interface{}, 0, len(possibleActions))
		for _, possibleAction := range possibleActions {
			ret = append(ret, possibleAction)
		}

		//fmt.Printf(
		//	"startingState:\n%s\n\npossibleActions:\n%s\n--------------------\n",
		//	startingState,
		//	maintenanceActionList(possibleActions),
		//)

		return ret
	}

	startTime := time.Now()
	//cameFrom, costSoFar, finalAction := DijkstraFindPath(
	cameFrom, costSoFar, finalAction := AStarFindPath(
		&DoNothingAction{finalState: startingState},
		coster,
		estimator,
		isGoaler,
		neighborGen,
	)
	runTime := time.Since(startTime)
	log.Printf("Plan generated in %s; total expansions %d; total cost %f\n", runTime, mp.expansions, costSoFar[finalAction])

	// if it didn't find any workable path, exit early
	if finalAction == nil {
		return nil
	}
	// rebuild the plan, working backwards from the final action
	var plan []MaintenanceAction
	currentAction := finalAction
	for currentAction != nil {
		plan = append(plan, currentAction.(MaintenanceAction))
		currentAction = cameFrom[currentAction]
	}
	// strip initial "DoNothingAction" from the plan
	plan = plan[:len(plan)-1]
	// reverse the plan, so that it's origin-first
	// see: https://github.com/golang/go/wiki/SliceTricks#reversing
	for i := len(plan)/2 - 1; i >= 0; i-- {
		opp := len(plan) - 1 - i
		plan[i], plan[opp] = plan[opp], plan[i]
	}

	return plan
}

type State []NodeState

func (s State) String() string {
	outB, err := yaml.Marshal(s)
	if err != nil {
		panic(err) // FIXME yuck.
	}
	return string(outB)
}

type NodeState struct {
	Name               string
	Cluster            int
	SoftwareRevision   int
	AppRunning         bool
	InLoadbalancerPool bool
	CacheWarmed        bool
}

type MaintenanceAction interface {
	fmt.Stringer
	CloneForValidTargets(startingState State) []MaintenanceAction
	FinalState() State
}

// used for debugging up in PlanActionsForTargetRevision
type maintenanceActionList []MaintenanceAction

func (mal maintenanceActionList) String() string {
	outStrings := make([]string, 0, len(mal))
	for _, action := range mal {
		outStrings = append(outStrings, action.String())
	}
	return strings.Join(outStrings, "\n")
}

type DoNothingAction struct {
	finalState State
}

func (dna DoNothingAction) String() string {
	return fmt.Sprintf("%T", dna)
}

func (dna DoNothingAction) CloneForValidTargets(startingState State) []MaintenanceAction {
	panic("never call me")
}

func (dna DoNothingAction) FinalState() State {
	return dna.finalState
}

type DrainNodeFromPoolAction struct {
	TargetRevision int
	finalState     State
	nodeName       string
}

func (dnfpa *DrainNodeFromPoolAction) String() string {
	return fmt.Sprintf("Drain node from pool: %s", dnfpa.nodeName)
}

func (dnfpa *DrainNodeFromPoolAction) CloneForValidTargets(startingState State) []MaintenanceAction {
	var out []MaintenanceAction

	downableCluster := getDownableCluster(startingState, dnfpa.TargetRevision)

	// clone for all nodes in the LB pool and in the "downable" cluster
	for i, nodeState := range startingState {
		nodeStep := stepNumberForNode(nodeState, dnfpa.TargetRevision)
		if nodeStep != 0 {
			continue
		}
		if nodeState.Cluster != downableCluster {
			continue
		}
		newNodeState := nodeState
		newNodeState.InLoadbalancerPool = false

		newState := make(State, len(startingState))
		copy(newState, startingState)
		newState[i] = newNodeState
		newAction := &DrainNodeFromPoolAction{
			nodeName:   newNodeState.Name,
			finalState: newState,
		}
		out = append(out, newAction)
	}
	return out
}

func (dnfpa *DrainNodeFromPoolAction) FinalState() State {
	return dnfpa.finalState
}

type StopAppAction struct {
	TargetRevision int
	nodeName       string
	finalState     State
}

func (sa *StopAppAction) String() string {
	return fmt.Sprintf("Stop app: %s", sa.nodeName)
}

func (sa *StopAppAction) CloneForValidTargets(startingState State) []MaintenanceAction {
	var out []MaintenanceAction

	downableCluster := getDownableCluster(startingState, sa.TargetRevision)

	// clone for all nodes not in the LB pool with running apps
	for i, nodeState := range startingState {
		nodeStep := stepNumberForNode(nodeState, sa.TargetRevision)
		if nodeStep != 1 {
			continue
		}
		lowStep := lowestStepForCluster(startingState, nodeState.Cluster, sa.TargetRevision)
		if lowStep < 1 {
			continue
		}
		if nodeState.Cluster != downableCluster {
			continue
		}
		newNodeState := nodeState
		newNodeState.AppRunning = false
		newNodeState.CacheWarmed = false

		newState := make(State, len(startingState))
		copy(newState, startingState)
		newState[i] = newNodeState
		newAction := &StopAppAction{
			nodeName:       newNodeState.Name,
			finalState:     newState,
			TargetRevision: sa.TargetRevision,
		}
		out = append(out, newAction)
	}
	return out
}

func (sa *StopAppAction) FinalState() State {
	return sa.finalState
}

type UpdateSoftwareRevisionAction struct {
	TargetRevision int
	finalState     State
	nodeName       string
}

func (usra *UpdateSoftwareRevisionAction) String() string {
	return fmt.Sprintf("Update software: %s", usra.nodeName)
}

func (usra *UpdateSoftwareRevisionAction) CloneForValidTargets(startingState State) []MaintenanceAction {
	var out []MaintenanceAction

	// clone for all nodes without running apps, running the wrong revision
	for i, nodeState := range startingState {
		nodeStep := stepNumberForNode(nodeState, usra.TargetRevision)
		if nodeStep != 2 {
			continue
		}
		lowStep := lowestStepForCluster(startingState, nodeState.Cluster, usra.TargetRevision)
		if lowStep < 2 {
			continue
		}
		newNodeState := nodeState
		newNodeState.SoftwareRevision = usra.TargetRevision

		newState := make(State, len(startingState))
		copy(newState, startingState)
		newState[i] = newNodeState
		newAction := &UpdateSoftwareRevisionAction{
			nodeName:   newNodeState.Name,
			finalState: newState,
		}
		out = append(out, newAction)
	}
	return out
}

func (usra *UpdateSoftwareRevisionAction) FinalState() State {
	return usra.finalState
}

type StartAppAction struct {
	TargetRevision int
	nodeName       string
	finalState     State
}

func (sa *StartAppAction) String() string {
	return fmt.Sprintf("Start app: %s", sa.nodeName)
}

func (sa *StartAppAction) CloneForValidTargets(startingState State) []MaintenanceAction {
	var out []MaintenanceAction

	// clone for all nodes without running apps
	for i, nodeState := range startingState {
		nodeStep := stepNumberForNode(nodeState, sa.TargetRevision)
		if nodeStep != 3 {
			continue
		}
		lowStep := lowestStepForCluster(startingState, nodeState.Cluster, sa.TargetRevision)
		if lowStep < 3 {
			continue
		}
		newNodeState := nodeState
		newNodeState.AppRunning = true
		newNodeState.CacheWarmed = false

		newState := make(State, len(startingState))
		copy(newState, startingState)
		newState[i] = newNodeState
		newAction := &StartAppAction{
			nodeName:       newNodeState.Name,
			finalState:     newState,
			TargetRevision: sa.TargetRevision,
		}
		out = append(out, newAction)
	}
	return out
}

func (sa *StartAppAction) FinalState() State {
	return sa.finalState
}

type WarmCacheAction struct {
	TargetRevision int
	finalState     State
	nodeName       string
}

func (wca *WarmCacheAction) String() string {
	return fmt.Sprintf("Warm cache: %s", wca.nodeName)
}

func (wca *WarmCacheAction) CloneForValidTargets(startingState State) []MaintenanceAction {
	var out []MaintenanceAction

	// clone for all nodes not in the LB pool, with running apps, and with cold caches
	for i, nodeState := range startingState {
		nodeStep := stepNumberForNode(nodeState, wca.TargetRevision)
		if nodeStep != 4 {
			continue
		}
		lowStep := lowestStepForCluster(startingState, nodeState.Cluster, wca.TargetRevision)
		if lowStep < 4 {
			continue
		}
		newNodeState := nodeState
		newNodeState.CacheWarmed = true

		newState := make(State, len(startingState))
		copy(newState, startingState)
		newState[i] = newNodeState
		newAction := &WarmCacheAction{
			nodeName:       newNodeState.Name,
			finalState:     newState,
			TargetRevision: wca.TargetRevision,
		}
		out = append(out, newAction)
	}
	return out
}

func (wca *WarmCacheAction) FinalState() State {
	return wca.finalState
}

type AddNodeToPoolAction struct {
	TargetRevision int
	finalState     State
	nodeName       string
}

func (antpa *AddNodeToPoolAction) String() string {
	return fmt.Sprintf("Add node to pool: %s", antpa.nodeName)
}

func (antpa *AddNodeToPoolAction) CloneForValidTargets(startingState State) []MaintenanceAction {
	var out []MaintenanceAction

	// clone for all nodes not in the LB pool with running app and cache warmed
	for i, nodeState := range startingState {
		nodeStep := stepNumberForNode(nodeState, antpa.TargetRevision)
		if nodeStep != 5 {
			continue
		}
		lowStep := lowestStepForCluster(startingState, nodeState.Cluster, antpa.TargetRevision)
		if lowStep < 5 {
			continue
		}

		newNodeState := nodeState
		newNodeState.InLoadbalancerPool = true

		newState := make(State, len(startingState))
		copy(newState, startingState)
		newState[i] = newNodeState
		newAction := &AddNodeToPoolAction{
			finalState:     newState,
			nodeName:       newNodeState.Name,
			TargetRevision: antpa.TargetRevision,
		}
		out = append(out, newAction)
	}
	return out
}

func (antpa *AddNodeToPoolAction) FinalState() State {
	return antpa.finalState
}

func baseEstimateForNode(nodeState NodeState, targetRevision int) float64 {
	var cost float64

	// calculate base cost on how many steps this node must absolutely complete
	if nodeState.SoftwareRevision != targetRevision {
		// at minimum have to upgrade the app, start it, warm the cache, and add to the pool
		cost += 4

		// have to drain it from the pool before upgrading
		// note that this is independent from stopping the app; we might be given
		// a node which is stopped yet somehow (?!) still in the pool
		if nodeState.InLoadbalancerPool {
			cost += 1
		}

		// have to stop the app before upgrading
		if nodeState.AppRunning {
			cost += 1
		}
	} else {
		if !nodeState.AppRunning {
			cost += 1
		}
		if !nodeState.CacheWarmed {
			cost += 1
		}
		if !nodeState.InLoadbalancerPool {
			cost += 1
		}
	}

	return cost
}

func estimateAction(action MaintenanceAction, targetRevision int) float64 {
	var maxCost float64
	for _, nodeState := range action.FinalState() {
		maxCost += baseEstimateForNode(nodeState, targetRevision)
	}

	return maxCost
}

// steps are these:
// 0- maintenance not started; node is in LB pool
// 1- maintenance started; node removed from LB pool
// 2- app stopped
// 3- software updated
// 4- app started
// 5- cache warmed
// 6- maintenance complete; node added back to LB pool
//
// Note that we're discarding invalid states here, e.g. the app isn't running
// but it's in the LB pool is treated as step 0/6; if treated as step 0 it'll
// end up correctly skipping stopping the app anyway
func stepNumberForNode(nodeState NodeState, targetRevision int) int {
	if nodeState.SoftwareRevision != targetRevision && nodeState.InLoadbalancerPool {
		return 0
	}
	if nodeState.SoftwareRevision != targetRevision && nodeState.AppRunning {
		return 1
	}
	if nodeState.SoftwareRevision != targetRevision {
		return 2
	}
	if !nodeState.AppRunning {
		return 3
	}
	if !nodeState.CacheWarmed {
		return 4
	}
	if !nodeState.InLoadbalancerPool {
		return 5
	}
	return 6
}

func lowestStepForCluster(state State, clusterNum, targetRevision int) int {
	lowestStep := math.MaxInt64
	for _, nodeState := range state {
		if nodeState.Cluster != clusterNum {
			continue
		}
		nodeStep := stepNumberForNode(nodeState, targetRevision)
		if nodeStep < lowestStep {
			lowestStep = nodeStep
		}
	}
	return lowestStep
}

func getDownableCluster(startingState State, targetRevision int) int {
	downClusters := make(map[int]bool)
	wrongRevClusters := make(map[int]bool)
	for _, nodeState := range startingState {
		if !nodeState.InLoadbalancerPool {
			downClusters[nodeState.Cluster] = true
		}
		if nodeState.SoftwareRevision != targetRevision {
			wrongRevClusters[nodeState.Cluster] = true
		}
	}
	// if more than one cluster is down then we can't proceed safely; return -1
	if len(downClusters) > 1 {
		return -1
	}
	// prefer any cluster which already has down nodes
	if len(downClusters) > 0 {
		for clusterNum := range downClusters {
			return clusterNum
		}
	}
	// fall back to lowest numbered cluster which has at least one node not at
	// the target revision
	if len(wrongRevClusters) > 0 {
		wrongRevClusterSlice := make([]int, 0, len(wrongRevClusters))
		for clusterNum := range wrongRevClusters {
			wrongRevClusterSlice = append(wrongRevClusterSlice, clusterNum)
		}
		sort.Ints(wrongRevClusterSlice)
		return wrongRevClusterSlice[0]
	}
	// otherwise return -1 to indicate that we shouldn't take any groups down,
	// because they're all up + at the target revision
	return -1
}
