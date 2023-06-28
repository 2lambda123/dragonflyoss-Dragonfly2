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

package gc

import (
	"context"
	"sync"
	"time"

	"d7y.io/dragonfly/v2/cdnsystem/config"
	"d7y.io/dragonfly/v2/cdnsystem/daemon/mgr"
	logger "d7y.io/dragonfly/v2/pkg/dflog"
)

func init() {
	// Ensure that Manager implements the GCMgr interface
	var manager *Manager = nil
	var _ mgr.GCMgr = manager
}

type Executor interface {
	GC(ctx context.Context) error
}

type ExecutorWrapper struct {
	gcInitialDelay time.Duration
	gcInterval     time.Duration
	gcExecutor     Executor
}

var (
	gcExecutorWrappers = make(map[string]*ExecutorWrapper)
)

func Register(name string, gcInitialDelay time.Duration, gcInterval time.Duration, gcExecutor Executor) {
	gcExecutorWrappers[name] = &ExecutorWrapper{
		gcInitialDelay: gcInitialDelay,
		gcInterval:     gcInterval,
		gcExecutor:     gcExecutor,
	}
}

// Manager is an implementation of the interface of GCMgr.
type Manager struct {
	cfg     *config.Config
	taskMgr mgr.SeedTaskMgr
	cdnMgr  mgr.CDNMgr
}

func (gcm *Manager) GCTask(ctx context.Context, taskID string, full bool) error {
	// todo data consistency
	var err error
	if full {
		err = gcm.cdnMgr.Delete(ctx, taskID)
		if err != nil {
			return err
		}
	}
	err = gcm.taskMgr.Delete(ctx, taskID)
	return err
}

// NewManager returns a new Manager.
func NewManager(cfg *config.Config, taskMgr mgr.SeedTaskMgr, cdnMgr mgr.CDNMgr) (*Manager, error) {
	return &Manager{
		cfg:     cfg,
		taskMgr: taskMgr,
		cdnMgr:  cdnMgr,
	}, nil
}

// StartGC starts to do the gc jobs.
func (gcm *Manager) StartGC(ctx context.Context) error {
	logger.Debugf("====start the gc jobs====")
	var wg sync.WaitGroup
	for name, executorWrapper := range gcExecutorWrappers {
		wg.Add(1)
		// start a goroutine to gc
		go func(name string, wrapper *ExecutorWrapper) {
			logger.Debugf("start the %s gc task", name)
			// delay to execute GC after gcm.initialDelay
			time.Sleep(wrapper.gcInitialDelay)
			wg.Done()
			// execute the GC by fixed delay
			ticker := time.NewTicker(wrapper.gcInterval)
			for {
				select {
				case <-ctx.Done():
					logger.Infof("exit %s gc task", name)
					return
				case <-ticker.C:
					wrapper.gcExecutor.GC(ctx)
				}
			}
		}(name, executorWrapper)
	}
	wg.Wait()
	logger.Debugf("====all gc jobs have been launched====")
	return nil
}
