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

package resource

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"

	"github.com/go-http-utils/headers"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	commonv1 "d7y.io/api/pkg/apis/common/v1"
	schedulerv1 "d7y.io/api/pkg/apis/scheduler/v1"
	"d7y.io/api/pkg/apis/scheduler/v1/mocks"
	"d7y.io/dragonfly/v2/client/util"
	"d7y.io/dragonfly/v2/pkg/idgen"
)

var (
	mockPeerID     = idgen.PeerID("127.0.0.1")
	mockSeedPeerID = idgen.SeedPeerID("127.0.0.1")
)

func TestPeer_NewPeer(t *testing.T) {
	tests := []struct {
		name    string
		id      string
		options []PeerOption
		expect  func(t *testing.T, peer *Peer, mockTask *Task, mockHost *Host)
	}{
		{
			name: "new peer",
			id:   mockPeerID,
			expect: func(t *testing.T, peer *Peer, mockTask *Task, mockHost *Host) {
				assert := assert.New(t)
				assert.Equal(peer.ID, mockPeerID)
				assert.Empty(peer.Pieces)
				assert.Equal(len(peer.PieceCosts()), 0)
				assert.Empty(peer.Stream)
				assert.Equal(peer.FSM.Current(), PeerStatePending)
				assert.EqualValues(peer.Task, mockTask)
				assert.EqualValues(peer.Host, mockHost)
				assert.NotEqual(peer.CreateAt.Load(), 0)
				assert.NotEqual(peer.UpdateAt.Load(), 0)
				assert.NotNil(peer.Log)
			},
		},
		{
			name:    "new peer with tag",
			id:      mockPeerID,
			options: []PeerOption{WithTag("foo")},
			expect: func(t *testing.T, peer *Peer, mockTask *Task, mockHost *Host) {
				assert := assert.New(t)
				assert.Equal(peer.ID, mockPeerID)
				assert.Equal(peer.Tag, "foo")
				assert.Empty(peer.Pieces)
				assert.Equal(len(peer.PieceCosts()), 0)
				assert.Empty(peer.Stream)
				assert.Equal(peer.FSM.Current(), PeerStatePending)
				assert.EqualValues(peer.Task, mockTask)
				assert.EqualValues(peer.Host, mockHost)
				assert.NotEqual(peer.CreateAt.Load(), 0)
				assert.NotEqual(peer.UpdateAt.Load(), 0)
				assert.NotNil(peer.Log)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mockHost := NewHost(mockRawHost)
			mockTask := NewTask(mockTaskID, mockTaskURL, commonv1.TaskType_Normal, mockTaskURLMeta, WithBackToSourceLimit(mockTaskBackToSourceLimit))
			tc.expect(t, NewPeer(tc.id, mockTask, mockHost, tc.options...), mockTask, mockHost)
		})
	}
}

func TestPeer_AppendPieceCost(t *testing.T) {
	tests := []struct {
		name   string
		expect func(t *testing.T, peer *Peer)
	}{
		{
			name: "append piece cost",
			expect: func(t *testing.T, peer *Peer) {
				assert := assert.New(t)
				peer.AppendPieceCost(1)
				costs := peer.PieceCosts()
				assert.Equal(costs[0], int64(1))
			},
		},
		{
			name: "piece costs slice is empty",
			expect: func(t *testing.T, peer *Peer) {
				assert := assert.New(t)
				costs := peer.PieceCosts()
				assert.Equal(len(costs), 0)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mockHost := NewHost(mockRawHost)
			mockTask := NewTask(mockTaskID, mockTaskURL, commonv1.TaskType_Normal, mockTaskURLMeta, WithBackToSourceLimit(mockTaskBackToSourceLimit))
			peer := NewPeer(mockPeerID, mockTask, mockHost)

			tc.expect(t, peer)
		})
	}
}

func TestPeer_PieceCosts(t *testing.T) {
	tests := []struct {
		name   string
		expect func(t *testing.T, peer *Peer)
	}{
		{
			name: "piece costs slice is not empty",
			expect: func(t *testing.T, peer *Peer) {
				assert := assert.New(t)
				peer.AppendPieceCost(1)
				costs := peer.PieceCosts()
				assert.Equal(costs[0], int64(1))
			},
		},
		{
			name: "piece costs slice is empty",
			expect: func(t *testing.T, peer *Peer) {
				assert := assert.New(t)
				costs := peer.PieceCosts()
				assert.Equal(len(costs), 0)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mockHost := NewHost(mockRawHost)
			mockTask := NewTask(mockTaskID, mockTaskURL, commonv1.TaskType_Normal, mockTaskURLMeta, WithBackToSourceLimit(mockTaskBackToSourceLimit))
			peer := NewPeer(mockPeerID, mockTask, mockHost)

			tc.expect(t, peer)
		})
	}
}

func TestPeer_LoadStream(t *testing.T) {
	tests := []struct {
		name   string
		expect func(t *testing.T, peer *Peer, stream schedulerv1.Scheduler_ReportPieceResultServer)
	}{
		{
			name: "load stream",
			expect: func(t *testing.T, peer *Peer, stream schedulerv1.Scheduler_ReportPieceResultServer) {
				assert := assert.New(t)
				peer.StoreStream(stream)
				newStream, ok := peer.LoadStream()
				assert.Equal(ok, true)
				assert.EqualValues(newStream, stream)
			},
		},
		{
			name: "stream does not exist",
			expect: func(t *testing.T, peer *Peer, stream schedulerv1.Scheduler_ReportPieceResultServer) {
				assert := assert.New(t)
				_, ok := peer.LoadStream()
				assert.Equal(ok, false)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctl := gomock.NewController(t)
			defer ctl.Finish()
			stream := mocks.NewMockScheduler_ReportPieceResultServer(ctl)

			mockHost := NewHost(mockRawHost)
			mockTask := NewTask(mockTaskID, mockTaskURL, commonv1.TaskType_Normal, mockTaskURLMeta, WithBackToSourceLimit(mockTaskBackToSourceLimit))
			peer := NewPeer(mockPeerID, mockTask, mockHost)
			tc.expect(t, peer, stream)
		})
	}
}

func TestPeer_StoreStream(t *testing.T) {
	tests := []struct {
		name   string
		expect func(t *testing.T, peer *Peer, stream schedulerv1.Scheduler_ReportPieceResultServer)
	}{
		{
			name: "store stream",
			expect: func(t *testing.T, peer *Peer, stream schedulerv1.Scheduler_ReportPieceResultServer) {
				assert := assert.New(t)
				peer.StoreStream(stream)
				newStream, ok := peer.LoadStream()
				assert.Equal(ok, true)
				assert.EqualValues(newStream, stream)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctl := gomock.NewController(t)
			defer ctl.Finish()
			stream := mocks.NewMockScheduler_ReportPieceResultServer(ctl)

			mockHost := NewHost(mockRawHost)
			mockTask := NewTask(mockTaskID, mockTaskURL, commonv1.TaskType_Normal, mockTaskURLMeta, WithBackToSourceLimit(mockTaskBackToSourceLimit))
			peer := NewPeer(mockPeerID, mockTask, mockHost)
			tc.expect(t, peer, stream)
		})
	}
}

func TestPeer_DeleteStream(t *testing.T) {
	tests := []struct {
		name   string
		expect func(t *testing.T, peer *Peer, stream schedulerv1.Scheduler_ReportPieceResultServer)
	}{
		{
			name: "delete stream",
			expect: func(t *testing.T, peer *Peer, stream schedulerv1.Scheduler_ReportPieceResultServer) {
				assert := assert.New(t)
				peer.StoreStream(stream)
				peer.DeleteStream()
				_, ok := peer.LoadStream()
				assert.Equal(ok, false)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctl := gomock.NewController(t)
			defer ctl.Finish()
			stream := mocks.NewMockScheduler_ReportPieceResultServer(ctl)

			mockHost := NewHost(mockRawHost)
			mockTask := NewTask(mockTaskID, mockTaskURL, commonv1.TaskType_Normal, mockTaskURLMeta, WithBackToSourceLimit(mockTaskBackToSourceLimit))
			peer := NewPeer(mockPeerID, mockTask, mockHost)
			tc.expect(t, peer, stream)
		})
	}
}

func TestPeer_Parents(t *testing.T) {
	tests := []struct {
		name   string
		expect func(t *testing.T, peer *Peer, seedPeer *Peer, stream schedulerv1.Scheduler_ReportPieceResultServer)
	}{
		{
			name: "peer has no parents",
			expect: func(t *testing.T, peer *Peer, seedPeer *Peer, stream schedulerv1.Scheduler_ReportPieceResultServer) {
				assert := assert.New(t)
				peer.Task.StorePeer(peer)
				assert.Equal(len(peer.Parents()), 0)
			},
		},
		{
			name: "peer has parents",
			expect: func(t *testing.T, peer *Peer, seedPeer *Peer, stream schedulerv1.Scheduler_ReportPieceResultServer) {
				assert := assert.New(t)
				peer.Task.StorePeer(peer)
				peer.Task.StorePeer(seedPeer)
				if err := peer.Task.AddPeerEdge(seedPeer, peer); err != nil {
					t.Fatal(err)
				}

				assert.Equal(len(peer.Parents()), 1)
				assert.Equal(peer.Parents()[0].ID, mockSeedPeerID)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctl := gomock.NewController(t)
			defer ctl.Finish()
			stream := mocks.NewMockScheduler_ReportPieceResultServer(ctl)

			mockHost := NewHost(mockRawHost)
			mockTask := NewTask(mockTaskID, mockTaskURL, commonv1.TaskType_Normal, mockTaskURLMeta, WithBackToSourceLimit(mockTaskBackToSourceLimit))
			peer := NewPeer(mockPeerID, mockTask, mockHost)
			seedPeer := NewPeer(mockSeedPeerID, mockTask, mockHost)
			tc.expect(t, peer, seedPeer, stream)
		})
	}
}

func TestPeer_Children(t *testing.T) {
	tests := []struct {
		name   string
		expect func(t *testing.T, peer *Peer, seedPeer *Peer, stream schedulerv1.Scheduler_ReportPieceResultServer)
	}{
		{
			name: "peer has no children",
			expect: func(t *testing.T, peer *Peer, seedPeer *Peer, stream schedulerv1.Scheduler_ReportPieceResultServer) {
				assert := assert.New(t)
				peer.Task.StorePeer(peer)
				assert.Equal(len(peer.Children()), 0)
			},
		},
		{
			name: "peer has children",
			expect: func(t *testing.T, peer *Peer, seedPeer *Peer, stream schedulerv1.Scheduler_ReportPieceResultServer) {
				assert := assert.New(t)
				peer.Task.StorePeer(peer)
				peer.Task.StorePeer(seedPeer)
				if err := peer.Task.AddPeerEdge(peer, seedPeer); err != nil {
					t.Fatal(err)
				}

				assert.Equal(len(peer.Children()), 1)
				assert.Equal(peer.Children()[0].ID, mockSeedPeerID)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctl := gomock.NewController(t)
			defer ctl.Finish()
			stream := mocks.NewMockScheduler_ReportPieceResultServer(ctl)

			mockHost := NewHost(mockRawHost)
			mockTask := NewTask(mockTaskID, mockTaskURL, commonv1.TaskType_Normal, mockTaskURLMeta, WithBackToSourceLimit(mockTaskBackToSourceLimit))
			peer := NewPeer(mockPeerID, mockTask, mockHost)
			seedPeer := NewPeer(mockSeedPeerID, mockTask, mockHost)
			tc.expect(t, peer, seedPeer, stream)
		})
	}
}

func TestPeer_DownloadTinyFile(t *testing.T) {
	testData := []byte("./0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz" +
		"./0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz")
	newServer := func(t *testing.T, getPeer func() *Peer) *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			peer := getPeer()
			assert := assert.New(t)
			assert.NotNil(peer)
			assert.Equal(r.URL.Path, fmt.Sprintf("/download/%s/%s", peer.Task.ID[:3], peer.Task.ID))
			assert.Equal(r.URL.RawQuery, fmt.Sprintf("peerId=%s", peer.ID))

			rgs, err := util.ParseRange(r.Header.Get(headers.Range), 128)
			assert.Nil(err)
			assert.Equal(1, len(rgs))
			rg := rgs[0]

			w.WriteHeader(http.StatusPartialContent)
			n, err := w.Write(testData[rg.Start : rg.Start+rg.Length])
			assert.Nil(err)
			assert.Equal(int64(n), rg.Length)
		}))
	}
	tests := []struct {
		name      string
		newServer func(t *testing.T, getPeer func() *Peer) *httptest.Server
		expect    func(t *testing.T, peer *Peer)
	}{
		{
			name: "download tiny file - 32",
			expect: func(t *testing.T, peer *Peer) {
				assert := assert.New(t)
				peer.Task.ContentLength.Store(32)
				data, err := peer.DownloadTinyFile()
				assert.NoError(err)
				assert.Equal(testData[:32], data)
			},
		},
		{
			name: "download tiny file - 128",
			expect: func(t *testing.T, peer *Peer) {
				assert := assert.New(t)
				peer.Task.ContentLength.Store(32)
				data, err := peer.DownloadTinyFile()
				assert.NoError(err)
				assert.Equal(testData[:32], data)
			},
		},
		{
			name: "download tiny file failed because of http status code",
			newServer: func(t *testing.T, getPeer func() *Peer) *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusNotFound)
				}))
			},
			expect: func(t *testing.T, peer *Peer) {
				assert := assert.New(t)
				peer.Task.ID = "foobar"
				_, err := peer.DownloadTinyFile()
				assert.EqualError(err, "bad response status 404 Not Found")
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var peer *Peer
			if tc.newServer == nil {
				tc.newServer = newServer
			}
			s := tc.newServer(t, func() *Peer {
				return peer
			})
			defer s.Close()
			url, err := url.Parse(s.URL)
			if err != nil {
				t.Fatal(err)
			}

			ip, rawPort, err := net.SplitHostPort(url.Host)
			if err != nil {
				t.Fatal(err)
			}

			port, err := strconv.ParseInt(rawPort, 10, 32)
			if err != nil {
				t.Fatal(err)
			}

			mockRawHost.Ip = ip
			mockRawHost.DownPort = int32(port)
			mockHost := NewHost(mockRawHost)
			mockTask := NewTask(mockTaskID, mockTaskURL, commonv1.TaskType_Normal, mockTaskURLMeta, WithBackToSourceLimit(mockTaskBackToSourceLimit))
			peer = NewPeer(mockPeerID, mockTask, mockHost)
			tc.expect(t, peer)
		})
	}
}
