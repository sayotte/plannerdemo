# Planner demo
## Overview
A "planner" is a tool which applies techniques from the
[planning and scheduling](https://en.wikipedia.org/wiki/Automated_planning_and_scheduling) branch
of AI to produce an ordered set of actions that will transition a system from a given starting state
to a desired end-state. 

This demo shows a planner which decides how to perform a rolling software upgrade across the
app-tier of an online service, fronted by a load-balancer, with zero downtime. The planner reacts
correctly to heterogenous starting states, e.g. some nodes are already updated, some are already
down for maintenance, and so on, while adhering to these rules:
1. Only nodes from one "cluster" may be down at any given time.
1. Nodes can only progress _forward_ towards their goal; they cannot regress to a previous state.
   1. This is a necessary performance constraint.
1. No nodes may be taken down in any cluster, if more than one cluster has nodes down.
   1. This is a safety feature.
   1. Due to the constraint from item #3 in this list, the planner is unable to produce any plan
      given a starting state with >1 cluster with "down" nodes.
      * If that constraint is relaxed the algorithm itself is very capable of deciding to "up" some
        nodes to get into a "safe" state. But as explained above, that constraint is necessary for
        performance reasons.  
      * Smarter solutions are very possible, I'll cover that in the **Lessons** section.
1. All nodes in a cluster must progress to the same maintenance "step" before any can move on to
   the next step.
   1. This is a nicety, to produce a plan which can be executed in parallel, but without forcing
      the planner itself to understand about that parallelism.
   1. E.g. this pseudocode could safely execute segments of the returned plan in parallel:
      ```
      nextActionType := plan[0].Type()
      var actions list[action]
      while(plan[0].Type() == nextActionType){
          nextAction := plan.lpop()
          list.append(nextAction)
      }
      spawnParallelWorkers(actions)
      ```
      This would e.g. perform all "Start app" actions in the same cluster at the same time, rather
      than sequentially.
## Installation and usage
##### Install
```sh
go get github.com/sayotte/plannerdemo
go install github.com/sayotte/plannerdemo/cmd/plannerdemo
``` 
##### Use
```sh
bin/plannerdemo -genStateFile
# Optionally, edit startingState.yaml to make interesting scenarios
bin/plannerdemo
```
##### Example output
```
main.go:121: Stop app: app2-2
main.go:121: Update software: app2-1
main.go:121: Update software: app2-2
main.go:121: Start app: app2-2
main.go:121: Start app: app2-1
main.go:121: Warm cache: app2-2
main.go:121: Warm cache: app2-1
main.go:121: Add node to pool: app2-2
main.go:121: Add node to pool: app2-1
main.go:121: Drain node from pool: app1-2
main.go:121: Drain node from pool: app1-1
main.go:121: Stop app: app1-2
main.go:121: Stop app: app1-1
main.go:121: Update software: app1-2
main.go:121: Update software: app1-1
main.go:121: Start app: app1-2
main.go:121: Start app: app1-1
main.go:121: Warm cache: app1-2
main.go:121: Warm cache: app1-1
main.go:121: Add node to pool: app1-2
main.go:121: Add node to pool: app1-1
```
##### Troubleshooting
There are only three cases in which the planner will fail to produce a plan:
1. All nodes are already at `softwarerevision: 2` (the target revision), so no actions are needed.
1. More than one "cluster" already has a node in a "down" state (i.e. either it's not in the load-balancer
   pool or its app is stopped). The planner treats this as an unsafe state and refuses to take additional
   nodes down.
   1. _"But why doesn't it just bring some nodes back up?"_ Good question, I'll explain that in the **Lessons**
   section below.
1. The system runs out of memory (or hits a ulimit).
   1. This can really happen. I'll cover more in the **Lessons** section, but during development I ran into
   this a lot. 
# Lessons
### Problem space
First, this problem falls into a straightforward class known as "Classical Planning Problems".
Borrowing from [Wikipedia](https://en.wikipedia.org/wiki/Automated_planning_and_scheduling#Overview),
the traits of a Classical Planning Problem are:
* a unique known initial state
* durationless actions,
* deterministic actions,
* which can be taken only one at a time,
* and a single agent.

If the planner needed to account _internally_ for parallel actions, or explicitly account for other planners
interacting with the same state, or included other more challenging variables it might fall into a
more complex class.

### Solution space
Many popular, modern solutions to Classical Planning problems descend from the [STRIPS planner](https://en.wikipedia.org/wiki/Stanford_Research_Institute_Problem_Solver),
which accepts a list of actions, each with a set of preconditions under which they're valid, and a set of
posconditions which they'll produce.

As a trivial example, consider how these actions could be combined into a plan to drink a glass of water:
```yaml
- action: grab glass
  preconditions:
    - glass not in hand
  postconditions:
    - glass in hand
- action: raise glass to mouth
  preconditions:
    - glass in hand
  postconditions:
    - glass at mouth
- action: drink water
  preconditions:
    - glass at mouth
  postconditions:
    - glass empty
    - water in belly 
```
The planner takes those actions, a starting state, and a goal-state (or a boolean function that can evaluate
whether a state satisfies the goal), and produces an ordered set of actions which achieve the goal-state.

This is accomplished internally by pairing a search algorithm with a node generator. The search algorithm
begins with a given starting state, and calls the generator to generate all valid actions from that
state. It then investigates the resulting states from each of those actions, and so on successively
until it reaches the goal.

Planners may consider non-deterministic action outcomes, parallel execution, and so on. They may be
primarily offline in which case time/space complexity is less important but reliable outcomes are
more important, or they may be called on to re-plan midway through execution in which case low
time/space complexity is more important but lower-confidence outcomes are fine. 
### Implementation
I pursued the STRIPS approach from scratch, using the [A* search algorithm](https://en.wikipedia.org/wiki/A*_search_algorithm),
coupled with a hand-built generator. I chose A* because heuristics can be applied to this problem,
and A* can produce optimal results with dramatically lower CPU / memory usage than breadth- or
depth-first algorithms. 

STRIPS planners frequently accept a generic input language (such as STRIPS' eponymous input
language, or lately [PDDL](https://en.wikipedia.org/wiki/Planning_Domain_Definition_Language)). I 
chose to forgoe that since this is just a proof-of-concept, but I'd probably do the same if I were
implementing something like this for work. The upside of recycling a known-good planner is dubious;
the core of this planner just a **tiny** amount of code, and the action-preconditions/-postconditions
are all functions rather than serializable states. Even assuming that were all possible, there'd
be a cost to translating to/from the generic language, and the potential for that translation to
distort how we see the problem.

Instead, I built actions as Go structs (analogous to classes in other languages), which have
methods for cloning themselves for all valid targets in a given state. This keeps the reasoning
about whether an action is valid very close to its declaration; also, in the other 
[learning project](https://github.com/sayotte/gomud2) of mine where I got this idea, I ended up
also adding methods to those structs to _execute_ the actions, keeping all of the logic close
together.
#### Complexity and mitigation
A*, given a good heuristic, approaches O(n) in both time and space complexity for a given fixed
graph it needs to traverse. But unlike the graph-traversal problems to which A* is normally
applied, we are generating the graph as we go, so "n" is not a fixed number.

In the worst case, the size of the graph we end up generating is proportional to (b^(x*y)), where:
* "b" is "branching factor", i.e. the number of actions we see as valid from any given state
* "x" is the number of server nodes we're dealing with
* "y" is the minimum number of actions each server node must go through to reach the goal

> I once consumed 64GB of RAM in only a few minutes, trying to find a solution
> for just a 15 server nodes in a single cluster.

To limit the size of the generated graph, I decided to constrain the branching-factor
by enforcing a rule that nodes can only move sequentially through "steps"-- this
essentially changes the branching factor to 1, and my space-complexity in runs is now generally in
the neighborhood of (x\*y*2).

##### Tradeoffs in chosen mitigation 
The "forward-only" rule came with one big tradeoff, though. Assume this starting state:
```yaml
node1:
  state: up
  cluster: 1
node2:
  state: down
  cluster: 1
node3:
  state: up
  cluster: 2
node4:
  state: down
  cluster: 2
``` 

Our rule #3 (see top of this README), added for safety, demands that we not take any more nodes
_down_ in this state. A human operator might suggest _"well, let's just bring either node2 or node4
back up, so that we're left with only one "down" cluster and can proceed"_, and indeed prior to
adding the "forward-only" rule the algorithm would happily produce exactly that solution. But the
addition of the "forward-only" rules prevents it from doing that, so it instead treats the above
starting state as insolvable.
#### Mitigation alternatives
##### Improved heuristic
In practice A* prunes most of the (B^(x*y)) graph with a good heuristic, where "good" is defined as
_"accurately estimates, but never overestimates, the distance-to-goal"_. An [unideal heuristic](https://en.wikipedia.org/wiki/A*_search_algorithm#Bounded_relaxation)
could prune dramatically more, although it wouldn't be guaranteed to produce the most efficient
plan in the end.

This seems attractive at first glance, and I did investigate it, but A* heuristics are quite subtle
and all of my attempts either _increased_ the search space or produced wildly inefficient
solutions.

I'd probably pursue this a bit farther if I were employing this professionally, just the same. 
##### Problem partitioning / sub-planners
The actions input into a planner could easily be planners in their own right. For instance, given
the starting state from above, one possible action might be a sub-planner called `BringClusterUp`,
which generates whatever actions are needed to bring all nodes in one cluster to the "up" state.

Since the sub-planner can have its own highly-restricted branching-factor, the overall branching
factor could be kept very low, while re-enabling us to "back-track" some nodes/clusters to a
healthy state before proceeding with maintenance on other nodes/clusters.

This seems overall like the most promising avenue, as it'd likely evolve into a suite of composable,
domain-specific behaviors that could be combined in novel ways by the planner, while being
developed and tested in isolation from one another.

# Caveats
This is a learning project, I started to see if I could apply some techniques I picked up in
[another learning project](https://github.com/sayotte/gomud2) to subjects closer to my professional
domain.

Please do take inspiration from this, but you really shouldn't re-use it without understanding it
a bit. I say that because the whole idea of a planner is to come up with novel solutions to
multi-dimensional problems. But, novel solutions are sometimes _surprising_ solutions, in the bad
sense. My favorite being:
> My goal is to not see any nodes going offline. So I turned them all off at once, and now nothing
> can go offline ever again. I have optimized for the FUTURE! 

And there's no quality control on any of this. I did write some test code, but that wasn't out of pride
or desire for presentability, much less quality... I simply needed to eliminate small logic errors
as the causes for failures I was trying to overcome.
 