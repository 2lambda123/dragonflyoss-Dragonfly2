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

package peer

import (
	"sync"
	"time"

	logger "d7y.io/dragonfly/v2/internal/dflog"
	"d7y.io/dragonfly/v2/scheduler/config"
	"d7y.io/dragonfly/v2/scheduler/supervisor"
)

type manager struct {
	hostManager              supervisor.HostMgr
	cleanupExpiredPeerTicker *time.Ticker
	peerTTL                  time.Duration
	peerTTI                  time.Duration
	peerMap                  sync.Map
	lock                     sync.RWMutex
}

func (m *manager) ListPeersByTask(taskID string) []*supervisor.Peer {
	var peers []*supervisor.Peer
	m.peerMap.Range(func(key, value interface{}) bool {
		peer := value.(*supervisor.Peer)
		if peer.Task.TaskID == taskID {
			peers = append(peers, peer)
		}
		return true
	})
	return peers
}

func (m *manager) ListPeers() *sync.Map {
	return &m.peerMap
}

func NewManager(cfg *config.GCConfig, hostManager supervisor.HostMgr) supervisor.PeerMgr {
	m := &manager{
		hostManager:              hostManager,
		cleanupExpiredPeerTicker: time.NewTicker(cfg.PeerGCInterval),
		peerTTL:                  cfg.PeerTTL,
		peerTTI:                  cfg.PeerTTI,
	}
	go m.cleanupPeers()
	return m
}

var _ supervisor.PeerMgr = (*manager)(nil)

func (m *manager) Add(peer *supervisor.Peer) {
	m.lock.Lock()
	defer m.lock.Unlock()
	peer.Host.AddPeer(peer)
	peer.Task.AddPeer(peer)
	m.peerMap.Store(peer.PeerID, peer)
}

func (m *manager) Get(peerID string) (*supervisor.Peer, bool) {
	data, ok := m.peerMap.Load(peerID)
	if !ok {
		return nil, false
	}
	peer := data.(*supervisor.Peer)
	return peer, true
}

func (m *manager) Delete(peerID string) {
	m.lock.Lock()
	defer m.lock.Unlock()
	peer, ok := m.Get(peerID)
	if ok {
		peer.Host.DeletePeer(peerID)
		peer.Task.DeletePeer(peer)
		peer.ReplaceParent(nil)
		m.peerMap.Delete(peerID)
	}
	return
}

func (m *manager) cleanupPeers() {
	for range m.cleanupExpiredPeerTicker.C {
		m.peerMap.Range(func(key, value interface{}) bool {
			peerID := key.(string)
			peer := value.(*supervisor.Peer)
			elapse := time.Since(peer.GetLastAccessTime())
			if elapse > m.peerTTI && !peer.IsDone() && !peer.Host.CDN {
				if !peer.IsConnected() {
					peer.MarkLeave()
				}
				logger.Debugf("peer %s has been more than %s since last access, set status to zombie", peer.PeerID, m.peerTTI)
				peer.SetStatus(supervisor.PeerStatusZombie)
			}
			if peer.IsLeave() || peer.IsFail() || elapse > m.peerTTL {
				if elapse > m.peerTTL {
					logger.Debugf("delete peer %s because %s have passed since last access", peer.PeerID)
				}
				m.Delete(peerID)
				if peer.Host.GetPeerTaskNum() == 0 {
					m.hostManager.Delete(peer.Host.UUID)
				}
				if peer.Task.ListPeers().Size() == 0 {
					peer.Task.SetStatus(supervisor.TaskStatusWaiting)
				}
			}
			return true
		})
	}
}
