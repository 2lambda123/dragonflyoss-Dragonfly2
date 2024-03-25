/*
 *     Copyright 2023 The Dragonfly Authors
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

package pex

import (
	"sync"

	dfdaemonv1 "d7y.io/api/v2/pkg/apis/dfdaemon/v1"

	logger "d7y.io/dragonfly/v2/internal/dflog"
)

type peerPool struct {
	lock *sync.RWMutex
	// map[task id] map[host id] *DestPeer
	tasks map[string]map[string]*DestPeer
}

func newPeerPool() *peerPool {
	return &peerPool{
		lock:  &sync.RWMutex{},
		tasks: map[string]map[string]*DestPeer{},
	}
}

func (p *peerPool) Sync(nodeMeta *MemberMeta, peerExchangeData *dfdaemonv1.PeerExchangeData) {
	p.lock.Lock()
	defer p.lock.Unlock()

	for _, peer := range peerExchangeData.PeerMetadatas {
		p.sync(nodeMeta, peer)
	}
}

func (p *peerPool) sync(nodeMeta *MemberMeta, peer *dfdaemonv1.PeerMetadata) {
	peers, ok := p.tasks[peer.TaskId]
	if !ok {
		peers = map[string]*DestPeer{}
		p.tasks[peer.TaskId] = peers
	}

	clean := func() {
		delete(peers, peer.PeerId)
		// clean empty task map
		if len(peers) == 0 {
			delete(p.tasks, peer.TaskId)
		}
	}

	switch peer.State {
	case dfdaemonv1.PeerState_Unknown:
		logger.Warnf("receive unknown state peer %s/%s from %s", peer.TaskId, peer.PeerId, nodeMeta.HostID)
		return
	case dfdaemonv1.PeerState_Running, dfdaemonv1.PeerState_Success:
		peers[nodeMeta.HostID] = &DestPeer{
			MemberMeta: nodeMeta,
			PeerID:     peer.PeerId,
		}
		logger.Infof("receive successful peer %s/%s from %s", peer.TaskId, peer.PeerId, nodeMeta.HostID)
	case dfdaemonv1.PeerState_Failed, dfdaemonv1.PeerState_Deleted:
		clean()
		logger.Infof("receive deleted peer %s/%s from %s", peer.TaskId, peer.PeerId, nodeMeta.HostID)
	default:
		logger.Warnf("receive unknown state peer %s/%s from %s", peer.TaskId, peer.PeerId, nodeMeta.HostID)
		return
	}
}

func (p *peerPool) Search(task string) SearchPeerResult {
	p.lock.RLock()
	defer p.lock.RUnlock()
	peers, ok := p.tasks[task]
	if !ok || len(peers) == 0 {
		return SearchPeerResult{
			Type: SearchPeerResultTypeNotFound,
		}
	}

	var (
		dp  []*DestPeer
		typ SearchPeerResultType = SearchPeerResultTypeRemote
	)
	for _, peer := range peers {
		// put local peer first
		if peer.IsLocal {
			typ = SearchPeerResultTypeLocal
			dp = append([]*DestPeer{peer}, dp...)
		} else {
			dp = append(dp, peer)
		}
	}

	// TODO check replica threshold and reclaim local cache

	return SearchPeerResult{
		Type:  typ,
		Peers: dp,
	}
}

func (p *peerPool) clean(hostID string) {
	for _, peers := range p.tasks {
		if _, ok := peers[hostID]; ok {
			delete(peers, hostID)
		}
	}
}