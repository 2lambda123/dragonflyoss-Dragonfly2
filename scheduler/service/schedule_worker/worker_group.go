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

package schedule_worker

import (
	"hash/crc32"

	logger "d7y.io/dragonfly/v2/pkg/dflog"
	scheduler2 "d7y.io/dragonfly/v2/pkg/rpc/scheduler"
	"d7y.io/dragonfly/v2/scheduler/config"
	"d7y.io/dragonfly/v2/scheduler/service"
	"d7y.io/dragonfly/v2/scheduler/types"
	"k8s.io/client-go/util/workqueue"
)

type IWorker interface {
	Serve()
	Stop()
	ReceiveJob(job *types.PeerTask)
	ReceiveUpdatePieceResult(pr *scheduler2.PieceResult)
}

type WorkerGroup struct {
	workerNum  int
	chanSize   int
	workerList []*Worker
	stopCh     chan struct{}
	sender     ISender

	triggerLoadQueue workqueue.Interface

	schedulerService *service.SchedulerService
}

func NewWorkerGroup(cfg *config.Config, schedulerService *service.SchedulerService) *WorkerGroup {
	return &WorkerGroup{
		workerNum:        cfg.Worker.WorkerNum,
		chanSize:         cfg.Worker.WorkerJobPoolSize,
		sender:           NewSender(cfg.Worker, schedulerService),
		schedulerService: schedulerService,
		triggerLoadQueue: workqueue.New(),
	}
}

func (wg *WorkerGroup) Serve() {
	wg.stopCh = make(chan struct{})

	wg.schedulerService.TaskManager.PeerTask.SetDownloadingMonitorCallBack(func(pt *types.PeerTask) {
		status := pt.GetNodeStatus()
		if status != types.PeerTaskStatusHealth {
			//} else if pt.GetNodeStatus() != types.PeerTaskStatusDone{
			//	return
		} else if pt.Success || pt.Host.Type == types.HostTypeCdn {
			return
		} else if pt.GetParent() == nil {
			pt.SetNodeStatus(types.PeerTaskStatusNeedParent)
		} else {
			pt.SetNodeStatus(types.PeerTaskStatusNeedCheckNode)
		}
		wg.ReceiveJob(pt)
	})

	for i := 0; i < wg.workerNum; i++ {
		w := NewWorker(wg.schedulerService, wg.sender, wg.ReceiveJob, wg.stopCh)
		w.Serve()
		wg.workerList = append(wg.workerList, w)
	}

	wg.sender.Serve()

	logger.Infof("start scheduler worker number:%d", wg.workerNum)
}

func (wg *WorkerGroup) Stop() {
	close(wg.stopCh)
	wg.sender.Stop()
	wg.triggerLoadQueue.ShutDown()
	logger.Infof("stop scheduler worker")
}

func (wg *WorkerGroup) ReceiveJob(job *types.PeerTask) {
	if job == nil {
		return
	}
	choiceWorkerId := crc32.ChecksumIEEE([]byte(job.Task.TaskId)) % uint32(wg.workerNum)
	wg.workerList[choiceWorkerId].ReceiveJob(job)
}

func (wg *WorkerGroup) ReceiveUpdatePieceResult(pr *scheduler2.PieceResult) {
	if pr == nil {
		return
	}
	choiceWorkerId := crc32.ChecksumIEEE([]byte(pr.SrcPid)) % uint32(wg.workerNum)
	wg.workerList[choiceWorkerId].ReceiveUpdatePieceResult(pr)
}
