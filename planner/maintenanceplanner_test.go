package planner

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"testing"
)

func Test_getDownableCluster(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		startingState  State
		targetRevision int
		expected       int
	}{
		"default to 1": {
			startingState: State{
				NodeState{
					Name:               "app1-1",
					Cluster:            1,
					SoftwareRevision:   1,
					AppRunning:         true,
					InLoadbalancerPool: true,
					CacheWarmed:        true,
				},
				NodeState{
					Name:               "app2-1",
					Cluster:            2,
					SoftwareRevision:   1,
					AppRunning:         true,
					InLoadbalancerPool: true,
					CacheWarmed:        true,
				},
			},
			targetRevision: 2,
			expected:       1,
		},
		"detect 2": {
			startingState: State{
				NodeState{
					Name:               "app1-1",
					Cluster:            1,
					SoftwareRevision:   1,
					AppRunning:         true,
					InLoadbalancerPool: true,
					CacheWarmed:        true,
				},
				NodeState{
					Name:               "app2-1",
					Cluster:            2,
					SoftwareRevision:   1,
					AppRunning:         false,
					InLoadbalancerPool: false,
					CacheWarmed:        false,
				},
			},
			targetRevision: 2,
			expected:       2,
		},
		"select 2 when 1 already at correct revision": {
			startingState: State{
				NodeState{
					Name:               "app1-1",
					Cluster:            1,
					SoftwareRevision:   2,
					AppRunning:         true,
					InLoadbalancerPool: true,
					CacheWarmed:        true,
				},
				NodeState{
					Name:               "app2-1",
					Cluster:            2,
					SoftwareRevision:   1,
					AppRunning:         false,
					InLoadbalancerPool: false,
					CacheWarmed:        false,
				},
			},
			targetRevision: 2,
			expected:       2,
		},
		"return -1 when more than one cluster is down": {
			startingState: State{
				NodeState{
					Name:               "app1-1",
					Cluster:            1,
					SoftwareRevision:   2,
					AppRunning:         false,
					InLoadbalancerPool: false,
					CacheWarmed:        false,
				},
				NodeState{
					Name:               "app2-1",
					Cluster:            2,
					SoftwareRevision:   1,
					AppRunning:         false,
					InLoadbalancerPool: false,
					CacheWarmed:        false,
				},
			},
			targetRevision: 2,
			expected:       -1,
		},
		"return -1 when all clusters up and at target rev": {
			startingState: State{
				NodeState{
					Name:               "app1-1",
					Cluster:            1,
					SoftwareRevision:   2,
					AppRunning:         true,
					InLoadbalancerPool: true,
					CacheWarmed:        true,
				},
				NodeState{
					Name:               "app2-1",
					Cluster:            2,
					SoftwareRevision:   2,
					AppRunning:         true,
					InLoadbalancerPool: true,
					CacheWarmed:        true,
				},
			},
			targetRevision: 2,
			expected:       -1,
		},
	}

	for testName, tc := range testCases {
		t.Run(testName, func(t *testing.T) {
			actual := getDownableCluster(tc.startingState, tc.targetRevision)
			if actual != tc.expected {
				t.Errorf("expected %d, got %d", tc.expected, actual)
			}
		})
	}
}

func Test_stepNumberForNode(t *testing.T) {
	t.Parallel()
	testCases := map[string]struct {
		nodeState      NodeState
		targetRevision int
		expected       int
	}{
		"step 0 unambiguous": {
			nodeState: NodeState{
				Name:               "app1-1",
				Cluster:            1,
				SoftwareRevision:   1,
				AppRunning:         true,
				InLoadbalancerPool: true,
				CacheWarmed:        true,
			},
			targetRevision: 2,
			expected:       0,
		},
		"step 0 ambiguous": {
			nodeState: NodeState{
				Name:               "app1-1",
				Cluster:            1,
				SoftwareRevision:   1,
				AppRunning:         false,
				InLoadbalancerPool: true,
				CacheWarmed:        false,
			},
			targetRevision: 2,
			expected:       0,
		},
		"step 1 unambiguous": {
			nodeState: NodeState{
				Name:               "app1-1",
				Cluster:            1,
				SoftwareRevision:   1,
				AppRunning:         true,
				InLoadbalancerPool: false,
				CacheWarmed:        true,
			},
			targetRevision: 2,
			expected:       1,
		},
		"step 2 unambiguous": {
			nodeState: NodeState{
				Name:               "app1-1",
				Cluster:            1,
				SoftwareRevision:   1,
				AppRunning:         false,
				InLoadbalancerPool: false,
				CacheWarmed:        false,
			},
			targetRevision: 2,
			expected:       2,
		},
		"step 3 unambiguous": {
			nodeState: NodeState{
				Name:               "app1-1",
				Cluster:            1,
				SoftwareRevision:   2,
				AppRunning:         false,
				InLoadbalancerPool: false,
				CacheWarmed:        false,
			},
			targetRevision: 2,
			expected:       3,
		},
		"step 4 unambiguous": {
			nodeState: NodeState{
				Name:               "app1-1",
				Cluster:            1,
				SoftwareRevision:   2,
				AppRunning:         true,
				InLoadbalancerPool: false,
				CacheWarmed:        false,
			},
			targetRevision: 2,
			expected:       4,
		},
		"step 5 unambiguous": {
			nodeState: NodeState{
				Name:               "app1-1",
				Cluster:            1,
				SoftwareRevision:   2,
				AppRunning:         true,
				InLoadbalancerPool: false,
				CacheWarmed:        true,
			},
			targetRevision: 2,
			expected:       5,
		},
		"step 6 unambiguous": {
			nodeState: NodeState{
				Name:               "app1-1",
				Cluster:            1,
				SoftwareRevision:   2,
				AppRunning:         true,
				InLoadbalancerPool: true,
				CacheWarmed:        true,
			},
			targetRevision: 2,
			expected:       6,
		},
	}

	for testName, tc := range testCases {
		t.Run(testName, func(t *testing.T) {
			actual := stepNumberForNode(tc.nodeState, tc.targetRevision)
			if actual != tc.expected {
				t.Errorf("expected %d, got %d", tc.expected, actual)
			}
		})
	}
}

func Test_lowestStepForCluster(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		startingState  State
		targetRevision int
		clusterNum     int
		expected       int
	}{
		"expect-0": {
			startingState: State{
				NodeState{
					Name:               "app1-1",
					Cluster:            1,
					SoftwareRevision:   1,
					AppRunning:         true,
					InLoadbalancerPool: true,
					CacheWarmed:        true,
				},
				NodeState{
					Name:               "app1-1",
					Cluster:            1,
					SoftwareRevision:   1,
					AppRunning:         true,
					InLoadbalancerPool: false,
					CacheWarmed:        true,
				},
			},
			targetRevision: 2,
			clusterNum:     1,
			expected:       0,
		},
	}

	for testName, tc := range testCases {
		t.Run(testName, func(t *testing.T) {
			actual := lowestStepForCluster(tc.startingState, tc.clusterNum, tc.targetRevision)
			if actual != tc.expected {
				t.Errorf("expected %d, got %d", tc.expected, actual)
			}
		})
	}
}

func ExampleMaintenancePlanner() {
	log.SetFlags(0)
	startingState := State{
		NodeState{
			Name:               "app1-1",
			Cluster:            1,
			SoftwareRevision:   1,
			AppRunning:         true,
			InLoadbalancerPool: true,
			CacheWarmed:        true,
		},
		NodeState{
			Name:               "app1-2",
			Cluster:            1,
			SoftwareRevision:   1,
			AppRunning:         true,
			InLoadbalancerPool: true,
			CacheWarmed:        true,
		},
		NodeState{
			Name:               "app2-1",
			Cluster:            2,
			SoftwareRevision:   1,
			AppRunning:         false,
			InLoadbalancerPool: false,
			CacheWarmed:        false,
		},
		NodeState{
			Name:               "app2-2",
			Cluster:            2,
			SoftwareRevision:   1,
			AppRunning:         true,
			InLoadbalancerPool: false,
			CacheWarmed:        true,
		},
	}

	log.SetOutput(ioutil.Discard)
	mp := &MaintenancePlanner{}
	plan := mp.PlanActionsForTargetRevision(startingState, 2)
	log.SetOutput(os.Stdout)

	for _,action := range plan {
		fmt.Println(action)
	}
	// Output:
	// Stop app: app2-2
	// Update software: app2-1
	// Update software: app2-2
	// Start app: app2-2
	// Start app: app2-1
	// Warm cache: app2-2
	// Warm cache: app2-1
	// Add node to pool: app2-2
	// Add node to pool: app2-1
	// Drain node from pool: app1-2
	// Drain node from pool: app1-1
	// Stop app: app1-2
	// Stop app: app1-1
	// Update software: app1-2
	// Update software: app1-1
	// Start app: app1-2
	// Start app: app1-1
	// Warm cache: app1-2
	// Warm cache: app1-1
	// Add node to pool: app1-2
	// Add node to pool: app1-1
}
