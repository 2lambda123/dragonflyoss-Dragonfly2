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

package d7y

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"sync"

	"d7y.io/dragonfly/v2/internal/dfcodes"
	"d7y.io/dragonfly/v2/internal/dferrors"
	logger "d7y.io/dragonfly/v2/internal/dflog"
	"d7y.io/dragonfly/v2/pkg/basic/dfnet"
	"d7y.io/dragonfly/v2/pkg/rpc/cdnsystem"
	"d7y.io/dragonfly/v2/pkg/rpc/cdnsystem/client"
	"d7y.io/dragonfly/v2/scheduler/config"
	"d7y.io/dragonfly/v2/scheduler/daemon"
	"d7y.io/dragonfly/v2/scheduler/types"
	"github.com/pkg/errors"
)

var ErrCDNRegisterFail = errors.New("cdn task register failed")

var ErrCDNDownloadFail = errors.New("cdn task download failed")

var ErrCDNUnknown = errors.New("cdn obtain seed encounter unknown err")

var ErrCDNInvokeFail = errors.New("invoke cdn interface failed")

var ErrInitCDNPeerFail = errors.New("init cdn peer failed")

type manager struct {
	client      client.CdnClient
	peerManager daemon.PeerMgr
	hostManager daemon.HostMgr
	lock        sync.RWMutex
}

func NewManager(cdnServers []*config.CDN, peerManager daemon.PeerMgr, hostManager daemon.HostMgr) (daemon.CDNMgr, error) {
	// Initialize CDNManager client
	cdnClient, err := client.GetClientByAddr(cdnHostsToNetAddrs(cdnServers))
	if err != nil {
		return nil, errors.Wrapf(err, "create cdn client for scheduler")
	}
	mgr := &manager{
		client:      cdnClient,
		peerManager: peerManager,
		hostManager: hostManager,
	}
	return mgr, nil
}

// cdnHostsToNetAddrs coverts manager.CdnHosts to []dfnet.NetAddr.
func cdnHostsToNetAddrs(hosts []*config.CDN) []dfnet.NetAddr {
	var netAddrs []dfnet.NetAddr
	for i := range hosts {
		netAddrs = append(netAddrs, dfnet.NetAddr{
			Type: dfnet.TCP,
			Addr: fmt.Sprintf("%s:%d", hosts[i].IP, hosts[i].Port),
		})
	}
	return netAddrs
}

func (cm *manager) OnNotify(c *config.DynconfigData) {
	cm.lock.Lock()
	defer cm.lock.Unlock()
	// Sync CDNManager client netAddrs
	cm.client.UpdateState(cdnHostsToNetAddrs(c.CDNs))
}

func (cm *manager) StartSeedTask(ctx context.Context, task *types.Task) error {
	if cm.client == nil {
		return ErrCDNRegisterFail
	}
	// TODO 这个地方必须重新生成一个ctx，不能使用传递进来的参数，需要排查下原因
	stream, err := cm.client.ObtainSeeds(context.Background(), &cdnsystem.SeedRequest{
		TaskId:  task.TaskID,
		Url:     task.URL,
		UrlMeta: task.URLMeta,
	})
	if err != nil {
		if cdnErr, ok := err.(*dferrors.DfError); ok {
			logger.Errorf("failed to obtain cdn seed: %v", cdnErr)
			switch cdnErr.Code {
			case dfcodes.CdnTaskRegistryFail:
				return errors.Wrap(ErrCDNRegisterFail, "obtain seeds")
			case dfcodes.CdnTaskDownloadFail:
				return errors.Wrapf(ErrCDNDownloadFail, "obtain seeds")
			default:
				return errors.Wrapf(ErrCDNUnknown, "obtain seeds")
			}
		}
		return errors.Wrapf(ErrCDNInvokeFail, "obtain seeds from cdn: %v", err)
	}
	return cm.receivePiece(task, stream)
}

func (cm *manager) receivePiece(task *types.Task, stream *client.PieceSeedStream) error {
	var once sync.Once
	var cdnPeer *types.Peer
	for {
		piece, err := stream.Recv()
		if err == io.EOF {
			if task.GetStatus() == types.TaskStatusSuccess {
				return nil
			}
			return errors.Errorf("cdn stream receive EOF but task status is %s", task.GetStatus())
		}
		if err != nil {
			if recvErr, ok := err.(*dferrors.DfError); ok {
				switch recvErr.Code {
				case dfcodes.CdnTaskRegistryFail:
					return errors.Wrapf(ErrCDNRegisterFail, "receive piece")
				case dfcodes.CdnTaskDownloadFail:
					return errors.Wrapf(ErrCDNDownloadFail, "receive piece")
				default:
					return errors.Wrapf(ErrCDNUnknown, "recive piece")
				}
			}
			return errors.Wrapf(ErrCDNInvokeFail, "receive piece from cdn: %v", err)
		}
		if piece != nil {
			once.Do(func() {
				cdnPeer, err = cm.initCdnPeer(task, piece)
			})
			if err != nil || cdnPeer == nil {
				return err
			}
			task.SetStatus(types.TaskStatusSeeding)
			cdnPeer.Touch()
			if piece.Done {
				task.PieceTotal = piece.TotalPieceCount
				task.ContentLength = piece.ContentLength
				task.SetStatus(types.TaskStatusSuccess)
				cdnPeer.SetStatus(types.PeerStatusSuccess)
				if task.ContentLength <= types.TinyFileSize {
					content, er := cm.DownloadTinyFileContent(task, cdnPeer.Host)
					if er == nil && len(content) == int(task.ContentLength) {
						task.DirectPiece = content
					}
				}
				return nil
			}
			cdnPeer.AddPieceInfo(piece.PieceInfo.PieceNum+1, 0)
			task.AddPiece(&types.PieceInfo{
				PieceNum:    piece.PieceInfo.PieceNum,
				RangeStart:  piece.PieceInfo.RangeStart,
				RangeSize:   piece.PieceInfo.RangeSize,
				PieceMd5:    piece.PieceInfo.PieceMd5,
				PieceOffset: piece.PieceInfo.PieceOffset,
				PieceStyle:  piece.PieceInfo.PieceStyle,
			})
		}
	}
}

func (cm *manager) initCdnPeer(task *types.Task, ps *cdnsystem.PieceSeed) (*types.Peer, error) {
	var ok bool
	var cdnHost *types.PeerHost
	cdnPeer, ok := cm.peerManager.Get(ps.PeerId)
	if !ok {
		logger.Debugf("first seed cdn task for taskID %s", task.TaskID)
		if cdnHost, ok = cm.hostManager.Get(ps.HostUuid); !ok {
			logger.Errorf("cannot find host %s", ps.HostUuid)
			return nil, errors.Wrapf(ErrInitCDNPeerFail, "cannot find host %s", ps.HostUuid)
		}
		cdnPeer = types.NewPeer(ps.PeerId, task, cdnHost)
	}
	cdnPeer.SetStatus(types.PeerStatusRunning)
	cm.peerManager.Add(cdnPeer)
	return cdnPeer, nil
}

func (cm *manager) DownloadTinyFileContent(task *types.Task, cdnHost *types.PeerHost) ([]byte, error) {
	// no need to invoke getPieceTasks method
	// TODO download the tiny file
	// http://host:port/download/{taskId 前3位}/{taskId}?peerId={peerId};
	url := fmt.Sprintf("http://%s:%d/download/%s/%s?peerId=scheduler",
		cdnHost.IP, cdnHost.DownloadPort, task.TaskID[:3], task.TaskID)
	response, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	content, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}

	return content, nil
}
