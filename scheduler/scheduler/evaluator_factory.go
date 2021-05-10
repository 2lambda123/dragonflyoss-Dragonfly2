/*
 *     Copyright 2020 The Dragonfly Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package scheduler

import (
	"sort"
	"strings"
	"sync"
	"time"

	"d7y.io/dragonfly/v2/pkg/safe"

	"d7y.io/dragonfly/v2/pkg/idgen"
	"d7y.io/dragonfly/v2/scheduler/config"
	"d7y.io/dragonfly/v2/scheduler/types"
)

type evaluatorFactory struct {
	lock                         *sync.RWMutex
	evaluators                   map[string]Evaluator
	getEvaluatorFuncs            map[int]getEvaluatorFunc
	getEvaluatorFuncPriorityList []getEvaluatorFunc
	cache                        map[*types.Task]Evaluator
	cacheClearFunc               *sync.Once
	abtest                       bool
	ascheduler                   string
	bscheduler                   string
}

type getEvaluatorFunc func(task *types.Task) (string, bool)

func newEvaluatorFactory(cfg config.SchedulerConfig) *evaluatorFactory {
	factory := &evaluatorFactory{
		lock:              new(sync.RWMutex),
		evaluators:        make(map[string]Evaluator),
		getEvaluatorFuncs: map[int]getEvaluatorFunc{},
		cache:             map[*types.Task]Evaluator{},
		cacheClearFunc:    new(sync.Once),
		abtest:            cfg.ABTest,
		ascheduler:        cfg.AScheduler,
		bscheduler:        cfg.BScheduler,
	}
	return factory
}

func (ef *evaluatorFactory) get(task *types.Task) Evaluator {
	ef.lock.RLock()

	evaluator, ok := ef.cache[task]
	if ok {
		ef.lock.RUnlock()
		return evaluator
	}

	if ef.abtest {
		name := ""
		if strings.HasSuffix(task.TaskId, idgen.TwinsBSuffix) {
			if ef.bscheduler != "" {
				name = ef.bscheduler
			}
		} else {
			if ef.ascheduler != "" {
				name = ef.ascheduler
			}
		}
		if name != "" {
			evaluator, ok = ef.evaluators[name]
			if ok {
				ef.lock.RUnlock()
				ef.lock.Lock()
				ef.cache[task] = evaluator
				ef.lock.Unlock()
				return evaluator
			}
		}
	}

	for _, fun := range ef.getEvaluatorFuncPriorityList {
		name, ok := fun(task)
		if !ok {
			continue
		}
		evaluator, ok = ef.evaluators[name]
		if !ok {
			continue
		}

		ef.lock.RUnlock()
		ef.lock.Lock()
		ef.cache[task] = evaluator
		ef.lock.Unlock()
		return evaluator
	}
	return nil
}

func (ef *evaluatorFactory) clearCache() {
	ef.lock.Lock()
	ef.cache = make(map[*types.Task]Evaluator)
	ef.lock.Unlock()
}

func (ef *evaluatorFactory) add(name string, evaluator Evaluator) {
	ef.lock.Lock()
	ef.evaluators[name] = evaluator
	ef.lock.Unlock()
}

func (ef *evaluatorFactory) addGetEvaluatorFunc(priority int, fun getEvaluatorFunc) {
	ef.lock.Lock()
	defer ef.lock.Unlock()
	_, ok := ef.getEvaluatorFuncs[priority]
	if ok {
		return
	}
	ef.getEvaluatorFuncs[priority] = fun
	var priorities []int
	for p := range ef.getEvaluatorFuncs {
		priorities = append(priorities, p)
	}
	sort.Ints(priorities)
	ef.getEvaluatorFuncPriorityList = ef.getEvaluatorFuncPriorityList[:0]
	for i := len(priorities) - 1; i >= 0; i-- {
		ef.getEvaluatorFuncPriorityList = append(ef.getEvaluatorFuncPriorityList, ef.getEvaluatorFuncs[priorities[i]])
	}

}

func (ef *evaluatorFactory) deleteGetEvaluatorFunc(priority int, fun getEvaluatorFunc) {
	ef.lock.Lock()

	delete(ef.getEvaluatorFuncs, priority)

	var priorities []int
	for p := range ef.getEvaluatorFuncs {
		priorities = append(priorities, p)
	}
	sort.Ints(priorities)
	ef.getEvaluatorFuncPriorityList = ef.getEvaluatorFuncPriorityList[:0]
	for i := len(priorities) - 1; i >= 0; i-- {
		ef.getEvaluatorFuncPriorityList = append(ef.getEvaluatorFuncPriorityList, ef.getEvaluatorFuncs[priorities[i]])
	}

	ef.lock.Unlock()
}

func (ef *evaluatorFactory) register(name string, evaluator Evaluator) {
	ef.cacheClearFunc.Do(func() {
		go safe.Call(func() {
			tick := time.NewTicker(time.Hour)
			for {
				select {
				case <-tick.C:
					ef.clearCache()
				}
			}
		})
	})
	ef.add(name, evaluator)
	ef.clearCache()
}

func (ef *evaluatorFactory) registerGetEvaluatorFunc(priority int, fun getEvaluatorFunc) {
	ef.addGetEvaluatorFunc(priority, fun)
	ef.clearCache()
}
