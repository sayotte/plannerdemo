package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"

	"gopkg.in/yaml.v2"

	"github.com/sayotte/plannerdemo/planner"
)


type cliArgs struct {
	startingStateFile string
	genStateFile bool
}

func parseArgs() cliArgs {
	startingStateFile := flag.String("stateFile", "startingState.yaml", "File containing starting state for planner; use -genStateFile to produce an example")
	genStateFile := flag.Bool("genStateFile", false, "Generate an example stateFile, then exit")
	flag.Parse()

	return cliArgs{
		startingStateFile: *startingStateFile,
		genStateFile: *genStateFile,
	}
}

func genStateFile(filename string) error {
	startingState := planner.State{
		planner.NodeState{
			Name:               "app1-1",
			Cluster:            1,
			SoftwareRevision:   1,
			AppRunning:         true,
			InLoadbalancerPool: true,
			CacheWarmed:        true,
		},
		planner.NodeState{
			Name:               "app1-2",
			Cluster:            1,
			SoftwareRevision:   1,
			AppRunning:         true,
			InLoadbalancerPool: true,
			CacheWarmed:        true,
		},
		planner.NodeState{
			Name:               "app2-1",
			Cluster:            2,
			SoftwareRevision:   1,
			AppRunning:         false,
			InLoadbalancerPool: false,
			CacheWarmed:        false,
		},
		planner.NodeState{
			Name:               "app2-2",
			Cluster:            2,
			SoftwareRevision:   1,
			AppRunning:         true,
			InLoadbalancerPool: false,
			CacheWarmed:        true,
		},
	}

	fd, err := os.OpenFile(filename, os.O_WRONLY|os.O_TRUNC|os.O_CREATE, 0644)
	if err != nil {
		return fmt.Errorf("os.OpenFile(%q): %s", filename, err)
	}
	defer func() { _ = fd.Close() }()
	outBytes, err := yaml.Marshal(startingState)
	if err != nil {
		return fmt.Errorf("yaml.Marshal: %s", err)
	}
	_, err = fd.Write(outBytes)
	if err != nil {
		return fmt.Errorf("fd.Write: %s", err)
	}
	return nil
}

func parseStateFile(filename string) (planner.State, error) {
	var startingState planner.State
	inBytes, err := ioutil.ReadFile(filename)
	if err != nil {
		return startingState, fmt.Errorf("ioutil.ReadFile(%q): %s", filename, err)
	}
	err = yaml.Unmarshal(inBytes, &startingState)
	if err != nil {
		return startingState, fmt.Errorf("yaml.Unmarshal: %s", err)
	}
	return startingState, nil
}

func main() {
	log.SetFlags(log.Lshortfile)

	args := parseArgs()

	if args.genStateFile {
		err := genStateFile(args.startingStateFile)
		if err != nil {
			log.Fatal(err)
		}
		return
	}

	startingState, err := parseStateFile(args.startingStateFile)
	if err != nil {
		log.Fatal(err)
	}

	mp := &planner.MaintenancePlanner{}
	plan := mp.PlanActionsForTargetRevision(startingState, 2)
	if len(plan) == 0 {
		log.Println("Empty plan returned.")
	}
	for _, action := range plan {
		log.Println(action)
	}
}
