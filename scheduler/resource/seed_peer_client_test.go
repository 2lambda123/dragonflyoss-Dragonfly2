/*
 *     Copyright 2022 The Dragonfly Authors
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
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	gomock "github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	"d7y.io/dragonfly/v2/manager/model"
	"d7y.io/dragonfly/v2/manager/types"
	"d7y.io/dragonfly/v2/pkg/dfnet"
	"d7y.io/dragonfly/v2/scheduler/config"
	configmocks "d7y.io/dragonfly/v2/scheduler/config/mocks"
)

func TestSeedPeerClient_newSeedPeerClient(t *testing.T) {
	tests := []struct {
		name   string
		mock   func(dynconfig *configmocks.MockDynconfigInterfaceMockRecorder, hostManager *MockHostManagerMockRecorder)
		expect func(t *testing.T, err error)
	}{
		{
			name: "new seed peer client",
			mock: func(dynconfig *configmocks.MockDynconfigInterfaceMockRecorder, hostManager *MockHostManagerMockRecorder) {
				gomock.InOrder(
					dynconfig.Get().Return(&config.DynconfigData{
						SeedPeers: []*config.SeedPeer{{ID: 1}},
					}, nil).Times(1),
					hostManager.Store(gomock.Any()).Return().Times(1),
					dynconfig.Register(gomock.Any()).Return().Times(1),
				)
			},
			expect: func(t *testing.T, err error) {
				assert := assert.New(t)
				assert.NoError(err)
			},
		},
		{
			name: "new seed peer client with cdn",
			mock: func(dynconfig *configmocks.MockDynconfigInterfaceMockRecorder, hostManager *MockHostManagerMockRecorder) {
				gomock.InOrder(
					dynconfig.Get().Return(&config.DynconfigData{
						CDNs: []*config.CDN{{ID: 1}},
					}, nil).Times(1),
					hostManager.Store(gomock.Any()).Return().Times(1),
					dynconfig.Register(gomock.Any()).Return().Times(1),
				)
			},
			expect: func(t *testing.T, err error) {
				assert := assert.New(t)
				assert.NoError(err)
			},
		},
		{
			name: "new seed peer client failed because of dynconfig get error data",
			mock: func(dynconfig *configmocks.MockDynconfigInterfaceMockRecorder, hostManager *MockHostManagerMockRecorder) {
				dynconfig.Get().Return(nil, errors.New("foo")).Times(1)
			},
			expect: func(t *testing.T, err error) {
				assert := assert.New(t)
				assert.EqualError(err, "foo")
			},
		},
		{
			name: "new seed peer client failed because of seed peer list is empty",
			mock: func(dynconfig *configmocks.MockDynconfigInterfaceMockRecorder, hostManager *MockHostManagerMockRecorder) {
				gomock.InOrder(
					dynconfig.Get().Return(&config.DynconfigData{
						SeedPeers: []*config.SeedPeer{},
					}, nil).Times(1),
				)
			},
			expect: func(t *testing.T, err error) {
				assert := assert.New(t)
				assert.EqualError(err, "address list of cdn is empty")
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctl := gomock.NewController(t)
			defer ctl.Finish()
			dynconfig := configmocks.NewMockDynconfigInterface(ctl)
			hostManager := NewMockHostManager(ctl)
			tc.mock(dynconfig.EXPECT(), hostManager.EXPECT())

			_, err := newSeedPeerClient(dynconfig, hostManager)
			tc.expect(t, err)
		})
	}
}

func TestSeedPeerClient_OnNotify(t *testing.T) {
	tests := []struct {
		name string
		data *config.DynconfigData
		mock func(dynconfig *configmocks.MockDynconfigInterfaceMockRecorder, hostManager *MockHostManagerMockRecorder)
	}{
		{
			name: "notify client without different seedPeers",
			data: &config.DynconfigData{
				SeedPeers: []*config.SeedPeer{{
					ID:       1,
					Hostname: "foo",
					IP:       "0.0.0.0",
					Port:     8080,
				}},
				CDNs: []*config.CDN{{
					ID:       1,
					Hostname: "foo",
					IP:       "0.0.0.0",
					Port:     8080,
				}},
			},
			mock: func(dynconfig *configmocks.MockDynconfigInterfaceMockRecorder, hostManager *MockHostManagerMockRecorder) {
				gomock.InOrder(
					dynconfig.Get().Return(&config.DynconfigData{
						SeedPeers: []*config.SeedPeer{{
							ID:       1,
							Hostname: "foo",
							IP:       "0.0.0.0",
							Port:     8080,
						}},
						CDNs: []*config.CDN{{
							ID:       1,
							Hostname: "foo",
							IP:       "0.0.0.0",
							Port:     8080,
						}},
					}, nil).Times(1),
					hostManager.Store(gomock.Any()).Return().Times(2),
					dynconfig.Register(gomock.Any()).Return().Times(1),
				)
			},
		},
		{
			name: "notify client with different seedPeers",
			data: &config.DynconfigData{
				SeedPeers: []*config.SeedPeer{{
					ID:       1,
					Hostname: "foo",
					IP:       "0.0.0.0",
				}},
				CDNs: []*config.CDN{{
					ID:       1,
					Hostname: "foo",
					IP:       "0.0.0.0",
				}},
			},
			mock: func(dynconfig *configmocks.MockDynconfigInterfaceMockRecorder, hostManager *MockHostManagerMockRecorder) {
				mockHost := NewHost(mockRawHost)
				gomock.InOrder(
					dynconfig.Get().Return(&config.DynconfigData{
						SeedPeers: []*config.SeedPeer{{
							ID:       1,
							Hostname: "foo",
							IP:       "127.0.0.1",
						}},
						CDNs: []*config.CDN{{
							ID:       1,
							Hostname: "foo",
							IP:       "127.0.0.1",
						}},
					}, nil).Times(1),
					hostManager.Store(gomock.Any()).Return().Times(2),
					dynconfig.Register(gomock.Any()).Return().Times(1),
					hostManager.Load(gomock.Any()).Return(mockHost, true).Times(1),
					hostManager.Delete(gomock.Eq("foo-0_Seed")).Return().Times(1),
					hostManager.Store(gomock.Any()).Return().Times(1),
					hostManager.Load(gomock.Any()).Return(mockHost, true).Times(1),
					hostManager.Delete(gomock.Eq("foo-0_CDN")).Return().Times(1),
					hostManager.Store(gomock.Any()).Return().Times(1),
				)
			},
		},
		{
			name: "notify client with different seed peers and load host failed",
			data: &config.DynconfigData{
				SeedPeers: []*config.SeedPeer{{
					ID:       1,
					Hostname: "foo",
					IP:       "0.0.0.0",
				}},
				CDNs: []*config.CDN{{
					ID:       1,
					Hostname: "foo",
					IP:       "0.0.0.0",
				}},
			},
			mock: func(dynconfig *configmocks.MockDynconfigInterfaceMockRecorder, hostManager *MockHostManagerMockRecorder) {
				gomock.InOrder(
					dynconfig.Get().Return(&config.DynconfigData{
						SeedPeers: []*config.SeedPeer{{
							ID:       1,
							Hostname: "foo",
							IP:       "127.0.0.1",
						}},
						CDNs: []*config.CDN{{
							ID:       1,
							Hostname: "foo",
							IP:       "127.0.0.1",
						}},
					}, nil).Times(1),
					hostManager.Store(gomock.Any()).Return().Times(2),
					dynconfig.Register(gomock.Any()).Return().Times(1),
					hostManager.Load(gomock.Any()).Return(nil, false).Times(1),
					hostManager.Store(gomock.Any()).Return().Times(1),
					hostManager.Load(gomock.Any()).Return(nil, false).Times(1),
					hostManager.Store(gomock.Any()).Return().Times(1),
				)
			},
		},
		{
			name: "seed peer list is deep equal",
			data: &config.DynconfigData{
				SeedPeers: []*config.SeedPeer{{
					ID: 1,
					IP: "127.0.0.1",
				}},
				CDNs: []*config.CDN{{
					ID: 1,
					IP: "127.0.0.1",
				}},
			},
			mock: func(dynconfig *configmocks.MockDynconfigInterfaceMockRecorder, hostManager *MockHostManagerMockRecorder) {
				gomock.InOrder(
					dynconfig.Get().Return(&config.DynconfigData{
						SeedPeers: []*config.SeedPeer{{
							ID: 1,
							IP: "127.0.0.1",
						}},
						CDNs: []*config.CDN{{
							ID: 1,
							IP: "127.0.0.1",
						}},
					}, nil).Times(1),
					hostManager.Store(gomock.Any()).Return().Times(2),
					dynconfig.Register(gomock.Any()).Return().Times(1),
				)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctl := gomock.NewController(t)
			defer ctl.Finish()
			dynconfig := configmocks.NewMockDynconfigInterface(ctl)
			hostManager := NewMockHostManager(ctl)
			tc.mock(dynconfig.EXPECT(), hostManager.EXPECT())

			client, err := newSeedPeerClient(dynconfig, hostManager)
			if err != nil {
				t.Fatal(err)
			}
			client.OnNotify(tc.data)
		})
	}
}

func TestSeedPeerClient_seedPeersToHosts(t *testing.T) {
	mockSeedPeerClusterConfig, err := json.Marshal(&types.SeedPeerClusterConfig{
		LoadLimit: 10,
	})
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name      string
		seedPeers []*config.SeedPeer
		expect    func(t *testing.T, hosts map[string]*Host)
	}{
		{
			name: "seed peers covert to hosts",
			seedPeers: []*config.SeedPeer{
				{
					ID:           1,
					Type:         model.SeedPeerTypeSuperSeed,
					Hostname:     mockRawSeedHost.HostName,
					IP:           mockRawSeedHost.Ip,
					Port:         mockRawSeedHost.RpcPort,
					DownloadPort: mockRawSeedHost.DownPort,
					IDC:          mockRawSeedHost.Idc,
					NetTopology:  mockRawSeedHost.NetTopology,
					Location:     mockRawSeedHost.Location,
					SeedPeerCluster: &config.SeedPeerCluster{
						Config: mockSeedPeerClusterConfig,
					},
				},
			},
			expect: func(t *testing.T, hosts map[string]*Host) {
				assert := assert.New(t)
				assert.Equal(hosts[mockRawSeedHost.Uuid].ID, mockRawSeedHost.Uuid)
				assert.Equal(hosts[mockRawSeedHost.Uuid].Type, HostTypeSuperSeed)
				assert.Equal(hosts[mockRawSeedHost.Uuid].IP, mockRawSeedHost.Ip)
				assert.Equal(hosts[mockRawSeedHost.Uuid].Hostname, mockRawSeedHost.HostName)
				assert.Equal(hosts[mockRawSeedHost.Uuid].Port, mockRawSeedHost.RpcPort)
				assert.Equal(hosts[mockRawSeedHost.Uuid].DownloadPort, mockRawSeedHost.DownPort)
				assert.Equal(hosts[mockRawSeedHost.Uuid].IDC, mockRawSeedHost.Idc)
				assert.Equal(hosts[mockRawSeedHost.Uuid].NetTopology, mockRawSeedHost.NetTopology)
				assert.Equal(hosts[mockRawSeedHost.Uuid].Location, mockRawSeedHost.Location)
				assert.Equal(hosts[mockRawSeedHost.Uuid].UploadLoadLimit.Load(), int32(10))
				assert.Empty(hosts[mockRawSeedHost.Uuid].Peers)
				assert.Equal(hosts[mockRawSeedHost.Uuid].IsCDN, false)
				assert.NotEqual(hosts[mockRawSeedHost.Uuid].CreateAt.Load(), 0)
				assert.NotEqual(hosts[mockRawSeedHost.Uuid].UpdateAt.Load(), 0)
				assert.NotNil(hosts[mockRawSeedHost.Uuid].Log)
			},
		},
		{
			name: "seed peers covert to hosts without cluster config",
			seedPeers: []*config.SeedPeer{
				{
					ID:           1,
					Type:         model.SeedPeerTypeSuperSeed,
					Hostname:     mockRawSeedHost.HostName,
					IP:           mockRawSeedHost.Ip,
					Port:         mockRawSeedHost.RpcPort,
					DownloadPort: mockRawSeedHost.DownPort,
					IDC:          mockRawSeedHost.Idc,
					NetTopology:  mockRawSeedHost.NetTopology,
					Location:     mockRawSeedHost.Location,
				},
			},
			expect: func(t *testing.T, hosts map[string]*Host) {
				assert := assert.New(t)
				assert.Equal(hosts[mockRawSeedHost.Uuid].ID, mockRawSeedHost.Uuid)
				assert.Equal(hosts[mockRawSeedHost.Uuid].Type, HostTypeSuperSeed)
				assert.Equal(hosts[mockRawSeedHost.Uuid].IP, mockRawSeedHost.Ip)
				assert.Equal(hosts[mockRawSeedHost.Uuid].Hostname, mockRawSeedHost.HostName)
				assert.Equal(hosts[mockRawSeedHost.Uuid].Port, mockRawSeedHost.RpcPort)
				assert.Equal(hosts[mockRawSeedHost.Uuid].DownloadPort, mockRawSeedHost.DownPort)
				assert.Equal(hosts[mockRawSeedHost.Uuid].IDC, mockRawSeedHost.Idc)
				assert.Equal(hosts[mockRawSeedHost.Uuid].NetTopology, mockRawSeedHost.NetTopology)
				assert.Equal(hosts[mockRawSeedHost.Uuid].Location, mockRawSeedHost.Location)
				assert.Equal(hosts[mockRawSeedHost.Uuid].UploadLoadLimit.Load(), int32(config.DefaultClientLoadLimit))
				assert.Empty(hosts[mockRawSeedHost.Uuid].Peers)
				assert.Equal(hosts[mockRawSeedHost.Uuid].IsCDN, false)
				assert.NotEqual(hosts[mockRawSeedHost.Uuid].CreateAt.Load(), 0)
				assert.NotEqual(hosts[mockRawSeedHost.Uuid].UpdateAt.Load(), 0)
				assert.NotNil(hosts[mockRawSeedHost.Uuid].Log)
			},
		},
		{
			name:      "seed peers is empty",
			seedPeers: []*config.SeedPeer{},
			expect: func(t *testing.T, hosts map[string]*Host) {
				assert := assert.New(t)
				assert.Equal(len(hosts), 0)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tc.expect(t, seedPeersToHosts(tc.seedPeers))
		})
	}
}

func TestSeedPeerClient_seedPeersToNetAddrs(t *testing.T) {
	tests := []struct {
		name      string
		seedPeers []*config.SeedPeer
		expect    func(t *testing.T, netAddrs []dfnet.NetAddr)
	}{
		{
			name: "seed peers covert to netAddr",
			seedPeers: []*config.SeedPeer{
				{
					ID:           1,
					Type:         model.SeedPeerTypeSuperSeed,
					Hostname:     mockRawSeedHost.HostName,
					IP:           mockRawSeedHost.Ip,
					Port:         mockRawSeedHost.RpcPort,
					DownloadPort: mockRawSeedHost.DownPort,
					IDC:          mockRawSeedHost.Idc,
					NetTopology:  mockRawSeedHost.NetTopology,
					Location:     mockRawSeedHost.Location,
				},
			},
			expect: func(t *testing.T, netAddrs []dfnet.NetAddr) {
				assert := assert.New(t)
				assert.Equal(netAddrs[0].Type, dfnet.TCP)
				assert.Equal(netAddrs[0].Addr, fmt.Sprintf("%s:%d", mockRawSeedHost.Ip, mockRawSeedHost.RpcPort))
			},
		},
		{
			name:      "seed peers is empty",
			seedPeers: []*config.SeedPeer{},
			expect: func(t *testing.T, netAddrs []dfnet.NetAddr) {
				assert := assert.New(t)
				assert.Equal(len(netAddrs), 0)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tc.expect(t, seedPeersToNetAddrs(tc.seedPeers))
		})
	}
}

func TestSeedPeerClient_diffSeedPeers(t *testing.T) {
	tests := []struct {
		name   string
		sx     []*config.SeedPeer
		sy     []*config.SeedPeer
		expect func(t *testing.T, diff []*config.SeedPeer)
	}{
		{
			name: "same seed peer list",
			sx: []*config.SeedPeer{
				{
					ID:       1,
					Hostname: "foo",
					IP:       "127.0.0.1",
					Port:     8080,
				},
			},
			sy: []*config.SeedPeer{
				{
					ID:       1,
					Hostname: "foo",
					IP:       "127.0.0.1",
					Port:     8080,
				},
			},
			expect: func(t *testing.T, diff []*config.SeedPeer) {
				assert := assert.New(t)
				assert.EqualValues(diff, []*config.SeedPeer(nil))
			},
		},
		{
			name: "different hostname",
			sx: []*config.SeedPeer{
				{
					ID:       1,
					Hostname: "bar",
					IP:       "127.0.0.1",
					Port:     8080,
				},
			},
			sy: []*config.SeedPeer{
				{
					ID:       1,
					Hostname: "foo",
					IP:       "127.0.0.1",
					Port:     8080,
				},
			},
			expect: func(t *testing.T, diff []*config.SeedPeer) {
				assert := assert.New(t)
				assert.EqualValues(diff, []*config.SeedPeer{
					{
						ID:       1,
						Hostname: "bar",
						IP:       "127.0.0.1",
						Port:     8080,
					},
				})
			},
		},
		{
			name: "different port",
			sx: []*config.SeedPeer{
				{
					ID:       1,
					Hostname: "foo",
					IP:       "127.0.0.1",
					Port:     8081,
				},
			},
			sy: []*config.SeedPeer{
				{
					ID:       1,
					Hostname: "foo",
					IP:       "127.0.0.1",
					Port:     8080,
				},
			},
			expect: func(t *testing.T, diff []*config.SeedPeer) {
				assert := assert.New(t)
				assert.EqualValues(diff, []*config.SeedPeer{
					{
						ID:       1,
						Hostname: "foo",
						IP:       "127.0.0.1",
						Port:     8081,
					},
				})
			},
		},
		{
			name: "different ip",
			sx: []*config.SeedPeer{
				{
					ID:       1,
					Hostname: "foo",
					IP:       "0.0.0.0",
					Port:     8080,
				},
			},
			sy: []*config.SeedPeer{
				{
					ID:       1,
					Hostname: "foo",
					IP:       "127.0.0.1",
					Port:     8080,
				},
			},
			expect: func(t *testing.T, diff []*config.SeedPeer) {
				assert := assert.New(t)
				assert.EqualValues(diff, []*config.SeedPeer{
					{
						ID:       1,
						Hostname: "foo",
						IP:       "0.0.0.0",
						Port:     8080,
					},
				})
			},
		},
		{
			name: "remove y seed peer",
			sx: []*config.SeedPeer{
				{
					ID:       1,
					Hostname: "foo",
					IP:       "127.0.0.1",
					Port:     8080,
				},
				{
					ID:       2,
					Hostname: "bar",
					IP:       "127.0.0.1",
					Port:     8080,
				},
			},
			sy: []*config.SeedPeer{
				{
					ID:       1,
					Hostname: "foo",
					IP:       "127.0.0.1",
					Port:     8080,
				},
			},
			expect: func(t *testing.T, diff []*config.SeedPeer) {
				assert := assert.New(t)
				assert.EqualValues(diff, []*config.SeedPeer{
					{
						ID:       2,
						Hostname: "bar",
						IP:       "127.0.0.1",
						Port:     8080,
					},
				})
			},
		},
		{
			name: "remove x seed peer",
			sx: []*config.SeedPeer{
				{
					ID:       1,
					Hostname: "foo",
					IP:       "127.0.0.1",
					Port:     8080,
				},
			},
			sy: []*config.SeedPeer{
				{
					ID:       1,
					Hostname: "baz",
					IP:       "127.0.0.1",
					Port:     8080,
				},
				{
					ID:       2,
					Hostname: "bar",
					IP:       "127.0.0.1",
					Port:     8080,
				},
			},
			expect: func(t *testing.T, diff []*config.SeedPeer) {
				assert := assert.New(t)
				assert.EqualValues(diff, []*config.SeedPeer{
					{
						ID:       1,
						Hostname: "foo",
						IP:       "127.0.0.1",
						Port:     8080,
					},
				})
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tc.expect(t, diffSeedPeers(tc.sx, tc.sy))
		})
	}
}

func TestSeedPeerClient_cdnsToHosts(t *testing.T) {
	mockCDNClusterConfig, err := json.Marshal(&types.CDNClusterConfig{
		LoadLimit:   10,
		NetTopology: "foo",
	})
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name   string
		cdns   []*config.CDN
		expect func(t *testing.T, hosts map[string]*Host)
	}{
		{
			name: "cdns covert to hosts",
			cdns: []*config.CDN{
				{
					ID:           1,
					Hostname:     mockRawCDNHost.HostName,
					IP:           mockRawCDNHost.Ip,
					Port:         mockRawCDNHost.RpcPort,
					DownloadPort: mockRawCDNHost.DownPort,
					Location:     mockRawCDNHost.Location,
					IDC:          mockRawCDNHost.Idc,
					CDNCluster: &config.CDNCluster{
						Config: mockCDNClusterConfig,
					},
				},
			},
			expect: func(t *testing.T, hosts map[string]*Host) {
				assert := assert.New(t)
				assert.Equal(hosts[mockRawCDNHost.Uuid].ID, mockRawCDNHost.Uuid)
				assert.Equal(hosts[mockRawCDNHost.Uuid].Type, HostTypeSuperSeed)
				assert.Equal(hosts[mockRawCDNHost.Uuid].IP, mockRawCDNHost.Ip)
				assert.Equal(hosts[mockRawCDNHost.Uuid].Hostname, mockRawCDNHost.HostName)
				assert.Equal(hosts[mockRawCDNHost.Uuid].Port, mockRawCDNHost.RpcPort)
				assert.Equal(hosts[mockRawCDNHost.Uuid].DownloadPort, mockRawCDNHost.DownPort)
				assert.Equal(hosts[mockRawCDNHost.Uuid].IDC, mockRawCDNHost.Idc)
				assert.Equal(hosts[mockRawCDNHost.Uuid].NetTopology, "foo")
				assert.Equal(hosts[mockRawCDNHost.Uuid].Location, mockRawCDNHost.Location)
				assert.Equal(hosts[mockRawCDNHost.Uuid].UploadLoadLimit.Load(), int32(10))
				assert.Empty(hosts[mockRawCDNHost.Uuid].Peers)
				assert.Equal(hosts[mockRawCDNHost.Uuid].IsCDN, true)
				assert.NotEqual(hosts[mockRawCDNHost.Uuid].CreateAt.Load(), 0)
				assert.NotEqual(hosts[mockRawCDNHost.Uuid].UpdateAt.Load(), 0)
				assert.NotNil(hosts[mockRawCDNHost.Uuid].Log)
			},
		},
		{
			name: "cdns covert to hosts without cluster config",
			cdns: []*config.CDN{
				{
					ID:           1,
					Hostname:     mockRawCDNHost.HostName,
					IP:           mockRawCDNHost.Ip,
					Port:         mockRawCDNHost.RpcPort,
					DownloadPort: mockRawCDNHost.DownPort,
					Location:     mockRawCDNHost.Location,
					IDC:          mockRawCDNHost.Idc,
				},
			},
			expect: func(t *testing.T, hosts map[string]*Host) {
				assert := assert.New(t)
				assert.Equal(hosts[mockRawCDNHost.Uuid].ID, mockRawCDNHost.Uuid)
				assert.Equal(hosts[mockRawCDNHost.Uuid].Type, HostTypeSuperSeed)
				assert.Equal(hosts[mockRawCDNHost.Uuid].IP, mockRawCDNHost.Ip)
				assert.Equal(hosts[mockRawCDNHost.Uuid].Hostname, mockRawCDNHost.HostName)
				assert.Equal(hosts[mockRawCDNHost.Uuid].Port, mockRawCDNHost.RpcPort)
				assert.Equal(hosts[mockRawCDNHost.Uuid].DownloadPort, mockRawCDNHost.DownPort)
				assert.Equal(hosts[mockRawCDNHost.Uuid].IDC, mockRawCDNHost.Idc)
				assert.Equal(hosts[mockRawCDNHost.Uuid].NetTopology, "")
				assert.Equal(hosts[mockRawCDNHost.Uuid].Location, mockRawCDNHost.Location)
				assert.Equal(hosts[mockRawCDNHost.Uuid].UploadLoadLimit.Load(), int32(config.DefaultClientLoadLimit))
				assert.Empty(hosts[mockRawCDNHost.Uuid].Peers)
				assert.Equal(hosts[mockRawCDNHost.Uuid].IsCDN, true)
				assert.NotEqual(hosts[mockRawCDNHost.Uuid].CreateAt.Load(), 0)
				assert.NotEqual(hosts[mockRawCDNHost.Uuid].UpdateAt.Load(), 0)
				assert.NotNil(hosts[mockRawCDNHost.Uuid].Log)
			},
		},
		{
			name: "cdns is empty",
			cdns: []*config.CDN{},
			expect: func(t *testing.T, hosts map[string]*Host) {
				assert := assert.New(t)
				assert.Equal(len(hosts), 0)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tc.expect(t, cdnsToHosts(tc.cdns))
		})
	}
}

func TestSeedPeerClient_cdnsToNetAddrs(t *testing.T) {
	tests := []struct {
		name   string
		cdns   []*config.CDN
		expect func(t *testing.T, netAddrs []dfnet.NetAddr)
	}{
		{
			name: "cdns covert to netAddr",
			cdns: []*config.CDN{
				{
					ID:           1,
					Hostname:     mockRawCDNHost.HostName,
					IP:           mockRawCDNHost.Ip,
					Port:         mockRawCDNHost.RpcPort,
					DownloadPort: mockRawCDNHost.DownPort,
					Location:     mockRawCDNHost.Location,
					IDC:          mockRawCDNHost.Idc,
				},
			},
			expect: func(t *testing.T, netAddrs []dfnet.NetAddr) {
				assert := assert.New(t)
				assert.Equal(netAddrs[0].Type, dfnet.TCP)
				assert.Equal(netAddrs[0].Addr, fmt.Sprintf("%s:%d", mockRawCDNHost.Ip, mockRawCDNHost.RpcPort))
			},
		},
		{
			name: "cdns is empty",
			cdns: []*config.CDN{},
			expect: func(t *testing.T, netAddrs []dfnet.NetAddr) {
				assert := assert.New(t)
				assert.Equal(len(netAddrs), 0)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tc.expect(t, cdnsToNetAddrs(tc.cdns))
		})
	}
}

func TestSeedPeerClient_diffCDNs(t *testing.T) {
	tests := []struct {
		name   string
		cx     []*config.CDN
		cy     []*config.CDN
		expect func(t *testing.T, diff []*config.CDN)
	}{
		{
			name: "same cdn list",
			cx: []*config.CDN{
				{
					ID:       1,
					Hostname: "foo",
					IP:       "127.0.0.1",
					Port:     8080,
				},
			},
			cy: []*config.CDN{
				{
					ID:       1,
					Hostname: "foo",
					IP:       "127.0.0.1",
					Port:     8080,
				},
			},
			expect: func(t *testing.T, diff []*config.CDN) {
				assert := assert.New(t)
				assert.EqualValues(diff, []*config.CDN(nil))
			},
		},
		{
			name: "different hostname",
			cx: []*config.CDN{
				{
					ID:       1,
					Hostname: "bar",
					IP:       "127.0.0.1",
					Port:     8080,
				},
			},
			cy: []*config.CDN{
				{
					ID:       1,
					Hostname: "foo",
					IP:       "127.0.0.1",
					Port:     8080,
				},
			},
			expect: func(t *testing.T, diff []*config.CDN) {
				assert := assert.New(t)
				assert.EqualValues(diff, []*config.CDN{
					{
						ID:       1,
						Hostname: "bar",
						IP:       "127.0.0.1",
						Port:     8080,
					},
				})
			},
		},
		{
			name: "different port",
			cx: []*config.CDN{
				{
					ID:       1,
					Hostname: "foo",
					IP:       "127.0.0.1",
					Port:     8081,
				},
			},
			cy: []*config.CDN{
				{
					ID:       1,
					Hostname: "foo",
					IP:       "127.0.0.1",
					Port:     8080,
				},
			},
			expect: func(t *testing.T, diff []*config.CDN) {
				assert := assert.New(t)
				assert.EqualValues(diff, []*config.CDN{
					{
						ID:       1,
						Hostname: "foo",
						IP:       "127.0.0.1",
						Port:     8081,
					},
				})
			},
		},
		{
			name: "different ip",
			cx: []*config.CDN{
				{
					ID:       1,
					Hostname: "foo",
					IP:       "0.0.0.0",
					Port:     8080,
				},
			},
			cy: []*config.CDN{
				{
					ID:       1,
					Hostname: "foo",
					IP:       "127.0.0.1",
					Port:     8080,
				},
			},
			expect: func(t *testing.T, diff []*config.CDN) {
				assert := assert.New(t)
				assert.EqualValues(diff, []*config.CDN{
					{
						ID:       1,
						Hostname: "foo",
						IP:       "0.0.0.0",
						Port:     8080,
					},
				})
			},
		},
		{
			name: "remove y cdn",
			cx: []*config.CDN{
				{
					ID:       1,
					Hostname: "foo",
					IP:       "127.0.0.1",
					Port:     8080,
				},
				{
					ID:       2,
					Hostname: "bar",
					IP:       "127.0.0.1",
					Port:     8080,
				},
			},
			cy: []*config.CDN{
				{
					ID:       1,
					Hostname: "foo",
					IP:       "127.0.0.1",
					Port:     8080,
				},
			},
			expect: func(t *testing.T, diff []*config.CDN) {
				assert := assert.New(t)
				assert.EqualValues(diff, []*config.CDN{
					{
						ID:       2,
						Hostname: "bar",
						IP:       "127.0.0.1",
						Port:     8080,
					},
				})
			},
		},
		{
			name: "remove x cdn",
			cx: []*config.CDN{
				{
					ID:       1,
					Hostname: "foo",
					IP:       "127.0.0.1",
					Port:     8080,
				},
			},
			cy: []*config.CDN{
				{
					ID:       1,
					Hostname: "baz",
					IP:       "127.0.0.1",
					Port:     8080,
				},
				{
					ID:       2,
					Hostname: "bar",
					IP:       "127.0.0.1",
					Port:     8080,
				},
			},
			expect: func(t *testing.T, diff []*config.CDN) {
				assert := assert.New(t)
				assert.EqualValues(diff, []*config.CDN{
					{
						ID:       1,
						Hostname: "foo",
						IP:       "127.0.0.1",
						Port:     8080,
					},
				})
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tc.expect(t, diffCDNs(tc.cx, tc.cy))
		})
	}
}
