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

package core

import (
	"context"
	"time"

	"d7y.io/dragonfly/v2/internal/dfcodes"
	"d7y.io/dragonfly/v2/internal/dferrors"
	logger "d7y.io/dragonfly/v2/internal/dflog"
	schedulerRPC "d7y.io/dragonfly/v2/pkg/rpc/scheduler"
	"d7y.io/dragonfly/v2/pkg/structure/sortedlist"
	"d7y.io/dragonfly/v2/scheduler/config"
	"d7y.io/dragonfly/v2/scheduler/core/scheduler"
	"d7y.io/dragonfly/v2/scheduler/supervisor"
	"go.opentelemetry.io/otel/trace"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/util/workqueue"
)

type event interface {
	hashKey() string
	apply(s *state)
}

type rsPeer struct {
	times        int32
	peer         *supervisor.Peer
	blankParents sets.String
}

type state struct {
	sched                       scheduler.Scheduler
	peerManager                 supervisor.PeerMgr
	cdnManager                  supervisor.CDNMgr
	waitScheduleParentPeerQueue workqueue.DelayingInterface
}

func newState(sched scheduler.Scheduler, peerManager supervisor.PeerMgr, cdnManager supervisor.CDNMgr, wsdq workqueue.DelayingInterface) *state {
	return &state{
		sched:                       sched,
		peerManager:                 peerManager,
		cdnManager:                  cdnManager,
		waitScheduleParentPeerQueue: wsdq,
	}
}

type reScheduleParentEvent struct {
	rsPeer *rsPeer
}

var _ event = reScheduleParentEvent{}

func (e reScheduleParentEvent) apply(s *state) {
	rsPeer := e.rsPeer
	rsPeer.times = rsPeer.times + 1
	peer := rsPeer.peer
	if peer.Task.IsFail() {
		if err := peer.CloseChannel(dferrors.New(dfcodes.SchedTaskStatusError, "schedule task status failed")); err != nil {
			logger.WithTaskAndPeerID(peer.Task.TaskID, peer.PeerID).Warnf("close peer channel failed: %v", err)
		}
		return
	}
	oldParent := peer.GetParent()
	blankParents := rsPeer.blankParents
	if oldParent != nil && !blankParents.Has(oldParent.PeerID) {
		logger.WithTaskAndPeerID(peer.Task.TaskID,
			peer.PeerID).Warnf("reScheduleParent： peer already schedule a parent %s and new parent is not in blank parents", oldParent.PeerID)
		return
	}
	parent, candidates, hasParent := s.sched.ScheduleParent(peer, blankParents)
	if !hasParent {
		if peer.Task.NeedClientBackSource() && !peer.Task.IsBackSourcePeer(peer.PeerID) {
			if peer.CloseChannel(dferrors.Newf(dfcodes.SchedNeedBackSource, "peer %s need back source", peer.PeerID)) == nil {
				peer.Task.IncreaseBackSourcePeer(peer.PeerID)
			}
			return
		}
		logger.Errorf("reScheduleParent: failed to schedule parent to peer %s, reschedule it later", peer.PeerID)
		s.waitScheduleParentPeerQueue.AddAfter(rsPeer, time.Second)
		return
	}
	// TODO if parentPeer is equal with oldParent, need schedule again ?
	if err := peer.SendSchedulePacket(constructSuccessPeerPacket(peer, parent, candidates)); err != nil {
		logger.WithTaskAndPeerID(peer.Task.TaskID, peer.PeerID).Warnf("send schedule packet to peer %s failed: %v", peer.PeerID, err)
	}
}

func (e reScheduleParentEvent) hashKey() string {
	return e.rsPeer.peer.Task.TaskID
}

type startReportPieceResultEvent struct {
	ctx  context.Context
	peer *supervisor.Peer
}

var _ event = startReportPieceResultEvent{}

func (e startReportPieceResultEvent) apply(s *state) {
	span := trace.SpanFromContext(e.ctx)
	parent := e.peer.GetParent()
	if parent != nil {
		logger.WithTaskAndPeerID(e.peer.Task.TaskID,
			e.peer.PeerID).Warnf("startReportPieceResultEvent: no need schedule parent because peer already had parent %s", parent.PeerID)
		if err := e.peer.SendSchedulePacket(constructSuccessPeerPacket(e.peer, parent, nil)); err != nil {
			logger.WithTaskAndPeerID(e.peer.Task.TaskID, e.peer.PeerID).Warnf("send schedule packet to peer failed: %v", err)
		}
		return
	}
	if e.peer.Task.IsBackSourcePeer(e.peer.PeerID) {
		logger.WithTaskAndPeerID(e.peer.Task.TaskID,
			e.peer.PeerID).Info("startReportPieceResultEvent: no need schedule parent because peer is back source peer")
		return
	}
	parent, candidates, hasParent := s.sched.ScheduleParent(e.peer, sets.NewString())
	// No parent node is currently available
	if !hasParent {
		if e.peer.Task.NeedClientBackSource() && !e.peer.Task.IsBackSourcePeer(e.peer.PeerID) {
			span.SetAttributes(config.AttributeClientBackSource.Bool(true))
			if e.peer.CloseChannel(dferrors.Newf(dfcodes.SchedNeedBackSource, "peer %s need back source", e.peer.PeerID)) == nil {
				e.peer.Task.IncreaseBackSourcePeer(e.peer.PeerID)
			}
			logger.WithTaskAndPeerID(e.peer.Task.TaskID,
				e.peer.PeerID).Info("startReportPieceResultEvent: peer need back source because no parent node is available for scheduling")
			return
		}
		logger.WithTaskAndPeerID(e.peer.Task.TaskID,
			e.peer.PeerID).Warnf("startReportPieceResultEvent: no parent node is currently available，reschedule it later")
		s.waitScheduleParentPeerQueue.AddAfter(&rsPeer{peer: e.peer}, time.Second)
		return
	}
	if err := e.peer.SendSchedulePacket(constructSuccessPeerPacket(e.peer, parent, candidates)); err != nil {
		logger.WithTaskAndPeerID(e.peer.Task.TaskID, e.peer.PeerID).Warnf("send schedule packet failed: %v", err)
	}
}

func (e startReportPieceResultEvent) hashKey() string {
	return e.peer.Task.TaskID
}

type peerDownloadPieceSuccessEvent struct {
	ctx  context.Context
	peer *supervisor.Peer
	pr   *schedulerRPC.PieceResult
}

var _ event = peerDownloadPieceSuccessEvent{}

func (e peerDownloadPieceSuccessEvent) apply(s *state) {
	e.peer.UpdateProgress(e.pr.FinishedCount, int(e.pr.EndTime-e.pr.BeginTime))
	if e.peer.Task.IsBackSourcePeer(e.peer.PeerID) {
		e.peer.Task.AddPiece(e.pr.PieceInfo)
		if !e.peer.Task.CanSchedule() {
			logger.WithTaskAndPeerID(e.peer.Task.TaskID,
				e.peer.PeerID).Warnf("peerDownloadPieceSuccessEvent: update task status seeding")
			e.peer.Task.SetStatus(supervisor.TaskStatusSeeding)
		}
		return
	}
	var candidates []*supervisor.Peer
	parentPeer, ok := s.peerManager.Get(e.pr.DstPid)
	if ok {
		oldParent := e.peer.GetParent()
		if e.pr.DstPid != e.peer.PeerID && (oldParent == nil || oldParent.PeerID != e.pr.DstPid) {
			logger.WithTaskAndPeerID(e.peer.Task.TaskID, e.peer.PeerID).Debugf("parent peerID is not same as DestPid, replace it's parent node with %s",
				e.pr.DstPid)
			e.peer.ReplaceParent(parentPeer)
		}
	} else if parentPeer.IsLeave() {
		logger.WithTaskAndPeerID(e.peer.Task.TaskID,
			e.peer.PeerID).Warnf("peerDownloadPieceSuccessEvent: need reschedule parent for peer because it's parent is already left")
		e.peer.ReplaceParent(nil)
		var hasParent bool
		parentPeer, candidates, hasParent = s.sched.ScheduleParent(e.peer, sets.NewString(parentPeer.PeerID))
		if !hasParent {
			logger.WithTaskAndPeerID(e.peer.Task.TaskID, e.peer.PeerID).Warnf("peerDownloadPieceSuccessEvent: no parent node is currently available, " +
				"reschedule it later")
			s.waitScheduleParentPeerQueue.AddAfter(&rsPeer{peer: e.peer, blankParents: sets.NewString(parentPeer.PeerID)}, time.Second)
			return
		}
	}
	parentPeer.Touch()
	if parentPeer.PeerID == e.pr.DstPid {
		return
	}
	// TODO if parentPeer is equal with oldParent, need schedule again ?
	if err := e.peer.SendSchedulePacket(constructSuccessPeerPacket(e.peer, parentPeer, candidates)); err != nil {
		logger.WithTaskAndPeerID(e.peer.Task.TaskID, e.peer.PeerID).Warnf("send schedule packet to peer %s failed: %v", e.peer.PeerID, err)
	}
}

func (e peerDownloadPieceSuccessEvent) hashKey() string {
	return e.peer.Task.TaskID
}

type peerDownloadPieceFailEvent struct {
	ctx  context.Context
	peer *supervisor.Peer
	pr   *schedulerRPC.PieceResult
}

var _ event = peerDownloadPieceFailEvent{}

func (e peerDownloadPieceFailEvent) apply(s *state) {
	if e.peer.Task.IsBackSourcePeer(e.peer.PeerID) {
		return
	}
	switch e.pr.Code {
	case dfcodes.ClientWaitPieceReady:
		return
	case dfcodes.PeerTaskNotFound:
		s.peerManager.Delete(e.pr.DstPid)
	case dfcodes.CdnTaskNotFound, dfcodes.CdnError, dfcodes.CdnTaskDownloadFail:
		s.peerManager.Delete(e.pr.DstPid)
		go func() {
			if _, err := s.cdnManager.StartSeedTask(e.ctx, e.peer.Task); err != nil {
				logger.WithTaskID(e.peer.Task.TaskID).Errorf("peerDownloadPieceFailEvent: seed task failed: %v", err)
			}
		}()
	default:
		logger.WithTaskAndPeerID(e.peer.Task.TaskID, e.peer.PeerID).Debugf("report piece download fail message, piece result %s", e.pr.String())
	}
	s.waitScheduleParentPeerQueue.Add(&rsPeer{peer: e.peer, blankParents: sets.NewString(e.pr.DstPid)})
}
func (e peerDownloadPieceFailEvent) hashKey() string {
	return e.peer.Task.TaskID
}

type taskSeedFailEvent struct {
	task *supervisor.Task
}

var _ event = taskSeedFailEvent{}

func (e taskSeedFailEvent) apply(s *state) {
	handleCDNSeedTaskFail(e.task)
}

func (e taskSeedFailEvent) hashKey() string {
	return e.task.TaskID
}

type peerDownloadSuccessEvent struct {
	peer       *supervisor.Peer
	peerResult *schedulerRPC.PeerResult
}

var _ event = peerDownloadSuccessEvent{}

func (e peerDownloadSuccessEvent) apply(s *state) {
	e.peer.SetStatus(supervisor.PeerStatusSuccess)
	if e.peer.Task.IsBackSourcePeer(e.peer.PeerID) && !e.peer.Task.IsSuccess() {
		e.peer.Task.UpdateTaskSuccessResult(e.peerResult.TotalPieceCount, e.peerResult.ContentLength)
	}
	removePeerFromCurrentTree(e.peer, s)
	children := s.sched.ScheduleChildren(e.peer, sets.NewString())
	for _, child := range children {
		if err := child.SendSchedulePacket(constructSuccessPeerPacket(child, e.peer, nil)); err != nil {
			logger.WithTaskAndPeerID(e.peer.Task.TaskID, e.peer.PeerID).Warnf("send schedule packet to peer %s failed: %v", child.PeerID, err)
		}
	}
}

func (e peerDownloadSuccessEvent) hashKey() string {
	return e.peer.Task.TaskID
}

type peerDownloadFailEvent struct {
	peer       *supervisor.Peer
	peerResult *schedulerRPC.PeerResult
}

var _ event = peerDownloadFailEvent{}

func (e peerDownloadFailEvent) apply(s *state) {
	e.peer.SetStatus(supervisor.PeerStatusFail)
	if e.peer.Task.IsBackSourcePeer(e.peer.PeerID) && !e.peer.Task.IsSuccess() {
		e.peer.Task.SetStatus(supervisor.TaskStatusFail)
		handleCDNSeedTaskFail(e.peer.Task)
		return
	}
	removePeerFromCurrentTree(e.peer, s)
	e.peer.GetChildren().Range(func(key, value interface{}) bool {
		child := (value).(*supervisor.Peer)
		parent, candidates, hasParent := s.sched.ScheduleParent(child, sets.NewString(e.peer.PeerID))
		if !hasParent {
			logger.WithTaskAndPeerID(child.Task.TaskID, child.PeerID).Warnf("peerDownloadFailEvent: there is no available parent, reschedule it later")
			s.waitScheduleParentPeerQueue.AddAfter(&rsPeer{peer: e.peer, blankParents: sets.NewString(e.peer.PeerID)}, time.Second)
			return true
		}
		if err := child.SendSchedulePacket(constructSuccessPeerPacket(child, parent, candidates)); err != nil {
			logger.WithTaskAndPeerID(e.peer.Task.TaskID, e.peer.PeerID).Warnf("send schedule packet to peer %s failed: %v", child.PeerID, err)
		}
		return true
	})
}

func (e peerDownloadFailEvent) hashKey() string {
	return e.peer.Task.TaskID
}

type peerLeaveEvent struct {
	ctx  context.Context
	peer *supervisor.Peer
}

var _ event = peerLeaveEvent{}

func (e peerLeaveEvent) apply(s *state) {
	e.peer.MarkLeave()
	removePeerFromCurrentTree(e.peer, s)
	e.peer.GetChildren().Range(func(key, value interface{}) bool {
		child := value.(*supervisor.Peer)
		parent, candidates, hasParent := s.sched.ScheduleParent(child, sets.NewString(e.peer.PeerID))
		if !hasParent {
			logger.WithTaskAndPeerID(child.Task.TaskID, child.PeerID).Warnf("handlePeerLeave: there is no available parent，reschedule it later")
			s.waitScheduleParentPeerQueue.AddAfter(&rsPeer{peer: child, blankParents: sets.NewString(e.peer.PeerID)}, time.Second)
			return true
		}
		if err := child.SendSchedulePacket(constructSuccessPeerPacket(child, parent, candidates)); err != nil {
			logger.WithTaskAndPeerID(e.peer.Task.TaskID, e.peer.PeerID).Warnf("send schedule packet to peer %s failed: %v", child.PeerID, err)
		}
		return true
	})
	s.peerManager.Delete(e.peer.PeerID)
}

func (e peerLeaveEvent) hashKey() string {
	return e.peer.Task.TaskID
}

// constructSuccessPeerPacket construct success peer schedule packet
func constructSuccessPeerPacket(peer *supervisor.Peer, parent *supervisor.Peer, candidates []*supervisor.Peer) *schedulerRPC.PeerPacket {
	mainPeer := &schedulerRPC.PeerPacket_DestPeer{
		Ip:      parent.Host.IP,
		RpcPort: parent.Host.RPCPort,
		PeerId:  parent.PeerID,
	}
	var stealPeers []*schedulerRPC.PeerPacket_DestPeer
	for _, candidate := range candidates {
		stealPeers = append(stealPeers, &schedulerRPC.PeerPacket_DestPeer{
			Ip:      candidate.Host.IP,
			RpcPort: candidate.Host.RPCPort,
			PeerId:  candidate.PeerID,
		})
	}
	peerPacket := &schedulerRPC.PeerPacket{
		TaskId:        peer.Task.TaskID,
		SrcPid:        peer.PeerID,
		ParallelCount: 1,
		MainPeer:      mainPeer,
		StealPeers:    stealPeers,
		Code:          dfcodes.Success,
	}
	logger.Debugf("send peerPacket %+v to peer %s", peerPacket, peer.PeerID)
	return peerPacket
}

func handleCDNSeedTaskFail(task *supervisor.Task) {
	if task.NeedClientBackSource() {
		task.ListPeers().Range(func(data sortedlist.Item) bool {
			peer := data.(*supervisor.Peer)
			if task.NeedClientBackSource() {
				if !task.IsBackSourcePeer(peer.PeerID) {
					if peer.CloseChannel(dferrors.Newf(dfcodes.SchedNeedBackSource, "peer %s need back source because cdn seed task failed", peer.PeerID)) == nil {
						task.IncreaseBackSourcePeer(peer.PeerID)
					}
				}
				return true
			}
			return false
		})
	} else {
		task.ListPeers().Range(func(data sortedlist.Item) bool {
			peer := data.(*supervisor.Peer)
			if err := peer.CloseChannel(dferrors.New(dfcodes.SchedTaskStatusError, "schedule task status failed")); err != nil {
				logger.WithTaskAndPeerID(peer.Task.TaskID, peer.PeerID).Warnf("close peer conn channel failed: %v", err)
			}
			return true
		})
	}
}

func removePeerFromCurrentTree(peer *supervisor.Peer, s *state) {
	parent := peer.GetParent()
	peer.ReplaceParent(nil)
	// parent frees up upload resources
	if parent != nil {
		children := s.sched.ScheduleChildren(parent, sets.NewString(peer.PeerID))
		for _, child := range children {
			if err := child.SendSchedulePacket(constructSuccessPeerPacket(child, peer, nil)); err != nil {
				logger.WithTaskAndPeerID(peer.Task.TaskID, peer.PeerID).Warnf("send schedule packet to peer %s failed: %v", child.PeerID, err)
			}
		}
	}
}
