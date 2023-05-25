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

package announcer

import (
	"bytes"
	"errors"
	"io"
	"net"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	trainerv1 "d7y.io/api/pkg/apis/trainer/v1"
	trainerv1mocks "d7y.io/api/pkg/apis/trainer/v1/mocks"

	pkgio "d7y.io/dragonfly/v2/pkg/io"
	managerclientmocks "d7y.io/dragonfly/v2/pkg/rpc/manager/client/mocks"
	trainerclientmocks "d7y.io/dragonfly/v2/pkg/rpc/trainer/client/mocks"
	"d7y.io/dragonfly/v2/scheduler/config"
	storagemocks "d7y.io/dragonfly/v2/scheduler/storage/mocks"
)

type mockReadCloserWithReadError struct{}

func (m *mockReadCloserWithReadError) Read(p []byte) (int, error) {
	return 0, errors.New("foo")
}

func (m *mockReadCloserWithReadError) Close() error {
	return nil
}

func TestAnnouncer_New(t *testing.T) {
	tests := []struct {
		name   string
		config *config.Config
		mock   func(m *managerclientmocks.MockV2MockRecorder)
		expect func(t *testing.T, announcer Announcer, err error)
	}{
		{
			name: "new announcer",
			config: &config.Config{
				Server: config.ServerConfig{
					Host:          "localhost",
					AdvertiseIP:   net.ParseIP("127.0.0.1"),
					AdvertisePort: 8004,
					Port:          8080,
				},
				Host: config.HostConfig{
					IDC:      "foo",
					Location: "bar",
				},
				Manager: config.ManagerConfig{
					SchedulerClusterID: 1,
				},
			},
			mock: func(m *managerclientmocks.MockV2MockRecorder) {
				m.UpdateScheduler(gomock.Any(), gomock.Any()).Return(nil, nil).Times(1)
			},
			expect: func(t *testing.T, a Announcer, err error) {
				assert := assert.New(t)
				instance := a.(*announcer)
				assert.NoError(err)
				assert.NotNil(instance.config)
				assert.NotNil(instance.managerClient)
			},
		},
		{
			name: "update scheduler failed",
			config: &config.Config{
				Server: config.ServerConfig{
					Host:          "localhost",
					AdvertiseIP:   net.ParseIP("127.0.0.1"),
					AdvertisePort: 8004,
					Port:          8080,
				},
				Host: config.HostConfig{
					IDC:      "foo",
					Location: "bar",
				},
				Manager: config.ManagerConfig{
					SchedulerClusterID: 1,
				},
			},
			mock: func(m *managerclientmocks.MockV2MockRecorder) {
				m.UpdateScheduler(gomock.Any(), gomock.Any()).Return(nil, errors.New("foo")).Times(1)
			},
			expect: func(t *testing.T, a Announcer, err error) {
				assert := assert.New(t)
				assert.Error(err)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctl := gomock.NewController(t)
			defer ctl.Finish()
			mockManagerClient := managerclientmocks.NewMockV2(ctl)
			mockStorage := storagemocks.NewMockStorage(ctl)
			tc.mock(mockManagerClient.EXPECT())

			a, err := New(tc.config, mockManagerClient, mockStorage)
			tc.expect(t, a, err)
		})
	}
}

func TestAnnouncer_Serve(t *testing.T) {
	ctl := gomock.NewController(t)
	defer ctl.Finish()
	mockTrainerClient := trainerclientmocks.NewMockV1(ctl)

	tests := []struct {
		name   string
		config *config.Config
		data   []byte
		option []Option
		sleep  func()
		mock   func(stream trainerv1.Trainer_TrainClient, data []byte, m *managerclientmocks.MockV2MockRecorder, mtc *trainerclientmocks.MockV1MockRecorder, ms *storagemocks.MockStorageMockRecorder, mt *trainerv1mocks.MockTrainer_TrainClientMockRecorder)
		except func(t *testing.T, a Announcer)
	}{
		{
			name: "started announcer server success",
			config: &config.Config{
				Server: config.ServerConfig{
					Host:          "localhost",
					AdvertiseIP:   net.ParseIP("127.0.0.1"),
					AdvertisePort: 8004,
					Port:          8080,
				},
				Host: config.HostConfig{
					IDC:      "foo",
					Location: "bar",
				},
				Manager: config.ManagerConfig{
					KeepAlive: config.KeepAliveConfig{
						Interval: 500 * time.Millisecond,
					},
					SchedulerClusterID: 1,
				},
				Trainer: config.TrainerConfig{
					Interval:      500 * time.Millisecond,
					UploadTimeout: 10 * time.Second,
				},
			},
			data:   []byte("buffer data"),
			option: []Option{WithTrainerClient(mockTrainerClient)},
			sleep: func() {
				time.Sleep(100 * time.Millisecond)
			},
			mock: func(stream trainerv1.Trainer_TrainClient, data []byte, m *managerclientmocks.MockV2MockRecorder, mtc *trainerclientmocks.MockV1MockRecorder, ms *storagemocks.MockStorageMockRecorder, mt *trainerv1mocks.MockTrainer_TrainClientMockRecorder) {
				m.UpdateScheduler(gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()
				m.KeepAlive(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
				mtc.Train(gomock.Any()).Return(stream, nil).AnyTimes()
				ms.OpenDownload().Return(io.NopCloser(bytes.NewBuffer(data)), nil).AnyTimes()
				ms.OpenNetworkTopology().Return(io.NopCloser(bytes.NewBuffer(data)), nil).AnyTimes()
				mt.Send(gomock.Any()).Return(nil).AnyTimes()
				mt.CloseAndRecv().Return(nil, nil).AnyTimes()
			},
			except: func(t *testing.T, a Announcer) {
				assert := assert.New(t)
				go func() {
					err := a.Serve()
					assert.NoError(err)
				}()
			},
		},
		{
			name: "started announcer server success without trainer client",
			config: &config.Config{
				Server: config.ServerConfig{
					Host:          "localhost",
					AdvertiseIP:   net.ParseIP("127.0.0.1"),
					AdvertisePort: 8004,
					Port:          8080,
				},
				Host: config.HostConfig{
					IDC:      "foo",
					Location: "bar",
				},
				Manager: config.ManagerConfig{
					KeepAlive: config.KeepAliveConfig{
						Interval: 500 * time.Millisecond,
					},
					SchedulerClusterID: 1,
				},
			},
			data:   []byte("buffer data"),
			option: []Option{},
			sleep: func() {
				time.Sleep(100 * time.Millisecond)
			},
			mock: func(stream trainerv1.Trainer_TrainClient, data []byte, m *managerclientmocks.MockV2MockRecorder, mtc *trainerclientmocks.MockV1MockRecorder, ms *storagemocks.MockStorageMockRecorder, mt *trainerv1mocks.MockTrainer_TrainClientMockRecorder) {
				m.UpdateScheduler(gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()
				m.KeepAlive(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
			},
			except: func(t *testing.T, a Announcer) {
				assert := assert.New(t)
				go func() {
					err := a.Serve()
					assert.NoError(err)
				}()
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			stream := trainerv1mocks.NewMockTrainer_TrainClient(ctl)
			mockManagerClient := managerclientmocks.NewMockV2(ctl)
			mockStorage := storagemocks.NewMockStorage(ctl)
			tc.mock(stream, tc.data, mockManagerClient.EXPECT(), mockTrainerClient.EXPECT(), mockStorage.EXPECT(), stream.EXPECT())
			a, err := New(tc.config, mockManagerClient, mockStorage, tc.option...)
			if err != nil {
				t.Fatal(err)
			}

			tc.except(t, a)
			tc.sleep()
			if err := a.Stop(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestAnnouncer_announceToManager(t *testing.T) {
	tests := []struct {
		name   string
		config *config.Config
		sleep  func()
		mock   func(m *managerclientmocks.MockV2MockRecorder)
		except func(t *testing.T, a Announcer)
	}{
		{
			name: "announce to manager success",
			config: &config.Config{
				Server: config.ServerConfig{
					Host:        "localhost",
					AdvertiseIP: net.ParseIP("127.0.0.1"),
				},
				Manager: config.ManagerConfig{
					KeepAlive: config.KeepAliveConfig{
						Interval: 500 * time.Millisecond,
					},
					SchedulerClusterID: 1,
				},
			},
			sleep: func() {
				time.Sleep(100 * time.Millisecond)
			},
			mock: func(m *managerclientmocks.MockV2MockRecorder) {
				m.UpdateScheduler(gomock.Any(), gomock.Any()).Return(nil, nil).Times(1)
				m.KeepAlive(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
			},
			except: func(t *testing.T, a Announcer) {
				assert := assert.New(t)
				err := a.(*announcer).announceToManager()
				assert.NoError(err)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctl := gomock.NewController(t)
			defer ctl.Finish()
			mockManagerClient := managerclientmocks.NewMockV2(ctl)
			mockTrainerClient := trainerclientmocks.NewMockV1(ctl)
			mockStorage := storagemocks.NewMockStorage(ctl)
			tc.mock(mockManagerClient.EXPECT())

			a, err := New(tc.config, mockManagerClient, mockStorage, WithTrainerClient(mockTrainerClient))
			if err != nil {
				t.Fatal(err)
			}
			tc.sleep()
			tc.except(t, a)
			if err := a.Stop(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestAnnouncer_announceToTrainer(t *testing.T) {
	tests := []struct {
		name   string
		config *config.Config
		data   []byte
		sleep  func()
		mock   func(stream trainerv1.Trainer_TrainClient, data []byte, m *managerclientmocks.MockV2MockRecorder, mtc *trainerclientmocks.MockV1MockRecorder, ms *storagemocks.MockStorageMockRecorder, mt *trainerv1mocks.MockTrainer_TrainClientMockRecorder)
		except func(t *testing.T, a Announcer)
	}{
		{
			name: "announce to trainer failed",
			config: &config.Config{
				Server: config.ServerConfig{
					Host:        "localhost",
					AdvertiseIP: net.ParseIP("127.0.0.1"),
				},
				Manager: config.ManagerConfig{
					SchedulerClusterID: 1,
				},
				Trainer: config.TrainerConfig{
					Interval:      50 * time.Millisecond,
					UploadTimeout: 10 * time.Second,
				},
			},
			data: []byte("buffer data"),
			sleep: func() {
				time.Sleep(100 * time.Millisecond)
			},
			mock: func(stream trainerv1.Trainer_TrainClient, data []byte, m *managerclientmocks.MockV2MockRecorder, mtc *trainerclientmocks.MockV1MockRecorder, ms *storagemocks.MockStorageMockRecorder, mt *trainerv1mocks.MockTrainer_TrainClientMockRecorder) {
				m.UpdateScheduler(gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()
				mtc.Train(gomock.Any()).Return(nil, errors.New("foo")).AnyTimes()
			},
			except: func(t *testing.T, a Announcer) {
				assert := assert.New(t)
				go func() {
					err := a.(*announcer).announceToTrainer()
					assert.EqualError(err, "foo")
				}()
			},
		},
		{
			name: "announce to trainer success",
			config: &config.Config{
				Server: config.ServerConfig{
					Host:        "localhost",
					AdvertiseIP: net.ParseIP("127.0.0.1"),
				},
				Manager: config.ManagerConfig{
					SchedulerClusterID: 1,
				},
				Trainer: config.TrainerConfig{
					Interval:      10 * time.Millisecond,
					UploadTimeout: 1 * time.Second,
				},
			},
			data: []byte("buffer data"),
			sleep: func() {
				time.Sleep(100 * time.Millisecond)
			},
			mock: func(stream trainerv1.Trainer_TrainClient, data []byte, m *managerclientmocks.MockV2MockRecorder, mtc *trainerclientmocks.MockV1MockRecorder, ms *storagemocks.MockStorageMockRecorder, mt *trainerv1mocks.MockTrainer_TrainClientMockRecorder) {
				m.UpdateScheduler(gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()
				mtc.Train(gomock.Any()).Return(stream, nil).AnyTimes()
				ms.OpenDownload().Return(io.NopCloser(bytes.NewBuffer(data)), nil).AnyTimes()
				ms.OpenNetworkTopology().Return(io.NopCloser(bytes.NewBuffer(data)), nil).AnyTimes()
				mt.Send(gomock.Any()).Return(nil).AnyTimes()
				mt.CloseAndRecv().Return(nil, nil).AnyTimes()
			},
			except: func(t *testing.T, a Announcer) {
				assert := assert.New(t)
				go func() {
					err := a.(*announcer).announceToTrainer()
					assert.NoError(err)
				}()
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctl := gomock.NewController(t)
			defer ctl.Finish()
			stream := trainerv1mocks.NewMockTrainer_TrainClient(ctl)
			mockManagerClient := managerclientmocks.NewMockV2(ctl)
			mockTrainerClient := trainerclientmocks.NewMockV1(ctl)
			mockStorage := storagemocks.NewMockStorage(ctl)
			tc.mock(stream, tc.data, mockManagerClient.EXPECT(), mockTrainerClient.EXPECT(), mockStorage.EXPECT(), stream.EXPECT())

			a, err := New(tc.config, mockManagerClient, mockStorage, WithTrainerClient(mockTrainerClient))
			if err != nil {
				t.Fatal(err)
			}
			tc.except(t, a)
			tc.sleep()
			if err := a.Stop(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestAnnouncer_train(t *testing.T) {
	tests := []struct {
		name   string
		config *config.Config
		data   []byte
		mock   func(stream trainerv1.Trainer_TrainClient, data []byte, m *managerclientmocks.MockV2MockRecorder, mtc *trainerclientmocks.MockV1MockRecorder, ms *storagemocks.MockStorageMockRecorder, mt *trainerv1mocks.MockTrainer_TrainClientMockRecorder)
		except func(t *testing.T, announcer Announcer, err error)
	}{
		{
			name: "get stream failed",
			config: &config.Config{
				Server: config.ServerConfig{
					Host:        "localhost",
					AdvertiseIP: net.ParseIP("127.0.0.1"),
				},
				Manager: config.ManagerConfig{
					SchedulerClusterID: 1,
				},
			},
			data: []byte("buffer data"),
			mock: func(stream trainerv1.Trainer_TrainClient, data []byte, m *managerclientmocks.MockV2MockRecorder, mtc *trainerclientmocks.MockV1MockRecorder, ms *storagemocks.MockStorageMockRecorder, mt *trainerv1mocks.MockTrainer_TrainClientMockRecorder) {
				m.UpdateScheduler(gomock.Any(), gomock.Any()).Return(nil, nil)
				mtc.Train(gomock.Any()).Return(nil, errors.New("foo"))
			},
			except: func(t *testing.T, a Announcer, err error) {
				assert := assert.New(t)
				assert.EqualError(err, "foo")
			},
		},
		{
			name: "upload download failed",
			config: &config.Config{
				Server: config.ServerConfig{
					Host:        "localhost",
					AdvertiseIP: net.ParseIP("127.0.0.1"),
				},
				Manager: config.ManagerConfig{
					SchedulerClusterID: 1,
				},
			},
			data: []byte("buffer data"),
			mock: func(stream trainerv1.Trainer_TrainClient, data []byte, m *managerclientmocks.MockV2MockRecorder, mtc *trainerclientmocks.MockV1MockRecorder, ms *storagemocks.MockStorageMockRecorder, mt *trainerv1mocks.MockTrainer_TrainClientMockRecorder) {
				m.UpdateScheduler(gomock.Any(), gomock.Any()).Return(nil, nil)
				mtc.Train(gomock.Any()).Return(stream, nil)
				ms.OpenDownload().Return(nil, errors.New("foo"))
				ms.OpenNetworkTopology().Return(io.NopCloser(bytes.NewBuffer(data)), nil)
				mt.Send(gomock.Any()).Return(nil).AnyTimes()
			},
			except: func(t *testing.T, a Announcer, err error) {
				assert := assert.New(t)
				assert.EqualError(err, "upload download: foo")
			},
		},
		{
			name: "upload network topology failed",
			config: &config.Config{
				Server: config.ServerConfig{
					Host:        "localhost",
					AdvertiseIP: net.ParseIP("127.0.0.1"),
				},
				Manager: config.ManagerConfig{
					SchedulerClusterID: 1,
				},
			},
			data: []byte("buffer data"),
			mock: func(stream trainerv1.Trainer_TrainClient, data []byte, m *managerclientmocks.MockV2MockRecorder, mtc *trainerclientmocks.MockV1MockRecorder, ms *storagemocks.MockStorageMockRecorder, mt *trainerv1mocks.MockTrainer_TrainClientMockRecorder) {
				m.UpdateScheduler(gomock.Any(), gomock.Any()).Return(nil, nil)
				mtc.Train(gomock.Any()).Return(stream, nil)
				ms.OpenDownload().Return(io.NopCloser(bytes.NewBuffer(data)), nil)
				ms.OpenNetworkTopology().Return(nil, errors.New("foo"))
				mt.Send(gomock.Any()).Return(nil).AnyTimes()
			},
			except: func(t *testing.T, a Announcer, err error) {
				assert := assert.New(t)
				assert.EqualError(err, "upload network topology: foo")
			},
		},
		{
			name: "close stream failed",
			config: &config.Config{
				Server: config.ServerConfig{
					Host:        "localhost",
					AdvertiseIP: net.ParseIP("127.0.0.1"),
				},
				Manager: config.ManagerConfig{
					SchedulerClusterID: 1,
				},
			},
			data: []byte("buffer data"),
			mock: func(stream trainerv1.Trainer_TrainClient, data []byte, m *managerclientmocks.MockV2MockRecorder, mtc *trainerclientmocks.MockV1MockRecorder, ms *storagemocks.MockStorageMockRecorder, mt *trainerv1mocks.MockTrainer_TrainClientMockRecorder) {
				m.UpdateScheduler(gomock.Any(), gomock.Any()).Return(nil, nil)
				mtc.Train(gomock.Any()).Return(stream, nil)
				ms.OpenDownload().Return(io.NopCloser(bytes.NewBuffer(data)), nil)
				ms.OpenNetworkTopology().Return(io.NopCloser(bytes.NewBuffer(data)), nil)
				mt.Send(gomock.Any()).Return(nil).AnyTimes()
				mt.CloseAndRecv().Return(nil, errors.New("foo"))
			},
			except: func(t *testing.T, a Announcer, err error) {
				assert := assert.New(t)
				assert.EqualError(err, "foo")
			},
		},
		{
			name: "train success",
			config: &config.Config{
				Server: config.ServerConfig{
					Host:        "localhost",
					AdvertiseIP: net.ParseIP("127.0.0.1"),
				},
				Manager: config.ManagerConfig{
					SchedulerClusterID: 1,
				},
			},
			data: []byte("buffer data"),
			mock: func(stream trainerv1.Trainer_TrainClient, data []byte, m *managerclientmocks.MockV2MockRecorder, mtc *trainerclientmocks.MockV1MockRecorder, ms *storagemocks.MockStorageMockRecorder, mt *trainerv1mocks.MockTrainer_TrainClientMockRecorder) {
				m.UpdateScheduler(gomock.Any(), gomock.Any()).Return(nil, nil)
				mtc.Train(gomock.Any()).Return(stream, nil)
				ms.OpenDownload().Return(io.NopCloser(bytes.NewBuffer(data)), nil)
				ms.OpenNetworkTopology().Return(io.NopCloser(bytes.NewBuffer(data)), nil)
				mt.Send(gomock.Any()).Return(nil).AnyTimes()
				mt.CloseAndRecv().Return(nil, nil)
			},
			except: func(t *testing.T, a Announcer, err error) {
				assert := assert.New(t)
				assert.NoError(err)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctl := gomock.NewController(t)
			defer ctl.Finish()
			stream := trainerv1mocks.NewMockTrainer_TrainClient(ctl)
			mockManagerClient := managerclientmocks.NewMockV2(ctl)
			mockTrainerClient := trainerclientmocks.NewMockV1(ctl)
			mockStorage := storagemocks.NewMockStorage(ctl)
			tc.mock(stream, tc.data, mockManagerClient.EXPECT(), mockTrainerClient.EXPECT(), mockStorage.EXPECT(), stream.EXPECT())

			a, err := New(tc.config, mockManagerClient, mockStorage, WithTrainerClient(mockTrainerClient))
			if err != nil {
				t.Fatal(err)
			}
			err = a.(*announcer).train()
			tc.except(t, a, err)
		})
	}
}

func TestAnnouncer_uploadDownloadToTrainer(t *testing.T) {
	tests := []struct {
		name   string
		config *config.Config
		data   []byte
		mock   func(stream trainerv1.Trainer_TrainClient, data []byte, m *managerclientmocks.MockV2MockRecorder, mtc *trainerclientmocks.MockV1MockRecorder, ms *storagemocks.MockStorageMockRecorder, mt *trainerv1mocks.MockTrainer_TrainClientMockRecorder)
		except func(t *testing.T, announcer Announcer, err error)
	}{
		{
			name: "open download failed",
			data: []byte{},
			config: &config.Config{
				Server: config.ServerConfig{
					Host:        "localhost",
					AdvertiseIP: net.ParseIP("127.0.0.1"),
				},
				Manager: config.ManagerConfig{
					SchedulerClusterID: 1,
				},
			},
			mock: func(stream trainerv1.Trainer_TrainClient, data []byte, m *managerclientmocks.MockV2MockRecorder, mtc *trainerclientmocks.MockV1MockRecorder, ms *storagemocks.MockStorageMockRecorder, mt *trainerv1mocks.MockTrainer_TrainClientMockRecorder) {
				gomock.InOrder(
					m.UpdateScheduler(gomock.Any(), gomock.Any()).Return(nil, nil).Times(1),
					ms.OpenDownload().Return(nil, errors.New("foo")),
				)
			},
			except: func(t *testing.T, a Announcer, err error) {
				assert := assert.New(t)
				assert.EqualError(err, "foo")
			},
		},
		{
			name: "read buffer failed",
			data: []byte{},
			config: &config.Config{
				Server: config.ServerConfig{
					Host:        "localhost",
					AdvertiseIP: net.ParseIP("127.0.0.1"),
				},
				Manager: config.ManagerConfig{
					SchedulerClusterID: 1,
				},
			},
			mock: func(stream trainerv1.Trainer_TrainClient, data []byte, m *managerclientmocks.MockV2MockRecorder, mtc *trainerclientmocks.MockV1MockRecorder, ms *storagemocks.MockStorageMockRecorder, mt *trainerv1mocks.MockTrainer_TrainClientMockRecorder) {
				gomock.InOrder(
					m.UpdateScheduler(gomock.Any(), gomock.Any()).Return(nil, nil).Times(1),
					ms.OpenDownload().Return(pkgio.MultiReadCloser(&mockReadCloserWithReadError{}), nil).Times(1),
				)
			},
			except: func(t *testing.T, a Announcer, err error) {
				assert := assert.New(t)
				assert.EqualError(err, "foo")
			},
		},
		{
			name: "send download request failed",
			data: []byte("download buffer"),
			config: &config.Config{
				Server: config.ServerConfig{
					Host:        "localhost",
					AdvertiseIP: net.ParseIP("127.0.0.1"),
				},
				Manager: config.ManagerConfig{
					SchedulerClusterID: 1,
				},
			},
			mock: func(stream trainerv1.Trainer_TrainClient, data []byte, m *managerclientmocks.MockV2MockRecorder, mtc *trainerclientmocks.MockV1MockRecorder, ms *storagemocks.MockStorageMockRecorder, mt *trainerv1mocks.MockTrainer_TrainClientMockRecorder) {
				gomock.InOrder(
					m.UpdateScheduler(gomock.Any(), gomock.Any()).Return(nil, nil).Times(1),
					ms.OpenDownload().Return(io.NopCloser(bytes.NewBuffer(data)), nil).Times(1),
					mt.Send(gomock.Any()).Return(errors.New("foo")),
				)
			},
			except: func(t *testing.T, a Announcer, err error) {
				assert := assert.New(t)
				assert.EqualError(err, "foo")
			},
		},
		{
			name: "send download request success",
			config: &config.Config{
				Server: config.ServerConfig{
					Host:        "localhost",
					AdvertiseIP: net.ParseIP("127.0.0.1"),
				},
				Manager: config.ManagerConfig{
					SchedulerClusterID: 1,
				},
			},
			data: []byte("download buffer"),
			mock: func(stream trainerv1.Trainer_TrainClient, data []byte, m *managerclientmocks.MockV2MockRecorder, mtc *trainerclientmocks.MockV1MockRecorder, ms *storagemocks.MockStorageMockRecorder, mt *trainerv1mocks.MockTrainer_TrainClientMockRecorder) {
				gomock.InOrder(
					m.UpdateScheduler(gomock.Any(), gomock.Any()).Return(nil, nil).Times(1),
					ms.OpenDownload().Return(io.NopCloser(bytes.NewBuffer(data)), nil).Times(1),
					mt.Send(gomock.Any()).DoAndReturn(
						func(t *trainerv1.TrainRequest) error {
							return nil
						}).AnyTimes(),
				)
			},
			except: func(t *testing.T, a Announcer, err error) {
				assert := assert.New(t)
				assert.NoError(err)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctl := gomock.NewController(t)
			defer ctl.Finish()
			stream := trainerv1mocks.NewMockTrainer_TrainClient(ctl)
			mockManagerClient := managerclientmocks.NewMockV2(ctl)
			mockTrainerClient := trainerclientmocks.NewMockV1(ctl)
			mockStorage := storagemocks.NewMockStorage(ctl)
			tc.mock(stream, tc.data, mockManagerClient.EXPECT(), mockTrainerClient.EXPECT(), mockStorage.EXPECT(), stream.EXPECT())

			a, err := New(tc.config, mockManagerClient, mockStorage, WithTrainerClient(mockTrainerClient))
			if err != nil {
				t.Fatal(err)
			}
			err = a.(*announcer).uploadDownloadToTrainer(stream)
			tc.except(t, a, err)
		})
	}
}

func TestAnnouncer_uploadNetworkTopologyToTrainer(t *testing.T) {
	tests := []struct {
		name   string
		config *config.Config
		data   []byte
		mock   func(stream trainerv1.Trainer_TrainClient, data []byte, m *managerclientmocks.MockV2MockRecorder, mtc *trainerclientmocks.MockV1MockRecorder, ms *storagemocks.MockStorageMockRecorder, mt *trainerv1mocks.MockTrainer_TrainClientMockRecorder)
		except func(t *testing.T, announcer Announcer, err error)
	}{
		{
			name: "open networkTopology failed",
			data: []byte{},
			config: &config.Config{
				Server: config.ServerConfig{
					Host:        "localhost",
					AdvertiseIP: net.ParseIP("127.0.0.1"),
				},
				Manager: config.ManagerConfig{
					SchedulerClusterID: 1,
				},
			},
			mock: func(stream trainerv1.Trainer_TrainClient, data []byte, m *managerclientmocks.MockV2MockRecorder, mtc *trainerclientmocks.MockV1MockRecorder, ms *storagemocks.MockStorageMockRecorder, mt *trainerv1mocks.MockTrainer_TrainClientMockRecorder) {
				gomock.InOrder(
					m.UpdateScheduler(gomock.Any(), gomock.Any()).Return(nil, nil).Times(1),
					ms.OpenNetworkTopology().Return(nil, errors.New("foo")),
				)
			},
			except: func(t *testing.T, a Announcer, err error) {
				assert := assert.New(t)
				assert.EqualError(err, "foo")
			},
		},
		{
			name: "read buffer failed",
			data: []byte{},
			config: &config.Config{
				Server: config.ServerConfig{
					Host:        "localhost",
					AdvertiseIP: net.ParseIP("127.0.0.1"),
				},
				Manager: config.ManagerConfig{
					SchedulerClusterID: 1,
				},
			},
			mock: func(stream trainerv1.Trainer_TrainClient, data []byte, m *managerclientmocks.MockV2MockRecorder, mtc *trainerclientmocks.MockV1MockRecorder, ms *storagemocks.MockStorageMockRecorder, mt *trainerv1mocks.MockTrainer_TrainClientMockRecorder) {
				gomock.InOrder(
					m.UpdateScheduler(gomock.Any(), gomock.Any()).Return(nil, nil).Times(1),
					ms.OpenNetworkTopology().Return(pkgio.MultiReadCloser(&mockReadCloserWithReadError{}), nil).Times(1),
				)
			},
			except: func(t *testing.T, a Announcer, err error) {
				assert := assert.New(t)
				assert.EqualError(err, "foo")
			},
		},
		{
			name: "send network topology failed",
			data: []byte("download buffer"),
			config: &config.Config{
				Server: config.ServerConfig{
					Host:        "localhost",
					AdvertiseIP: net.ParseIP("127.0.0.1"),
				},
				Manager: config.ManagerConfig{
					SchedulerClusterID: 1,
				},
			},
			mock: func(stream trainerv1.Trainer_TrainClient, data []byte, m *managerclientmocks.MockV2MockRecorder, mtc *trainerclientmocks.MockV1MockRecorder, ms *storagemocks.MockStorageMockRecorder, mt *trainerv1mocks.MockTrainer_TrainClientMockRecorder) {
				gomock.InOrder(
					m.UpdateScheduler(gomock.Any(), gomock.Any()).Return(nil, nil).Times(1),
					ms.OpenNetworkTopology().Return(io.NopCloser(bytes.NewBuffer(data)), nil).Times(1),
					mt.Send(gomock.Any()).Return(errors.New("foo")),
				)
			},
			except: func(t *testing.T, a Announcer, err error) {
				assert := assert.New(t)
				assert.EqualError(err, "foo")
			},
		},
		{
			name: "send network topology success",
			config: &config.Config{
				Server: config.ServerConfig{
					Host:        "localhost",
					AdvertiseIP: net.ParseIP("127.0.0.1"),
				},
				Manager: config.ManagerConfig{
					SchedulerClusterID: 1,
				},
			},
			data: []byte("networkTopology buffer"),
			mock: func(stream trainerv1.Trainer_TrainClient, data []byte, m *managerclientmocks.MockV2MockRecorder, mtc *trainerclientmocks.MockV1MockRecorder, ms *storagemocks.MockStorageMockRecorder, mt *trainerv1mocks.MockTrainer_TrainClientMockRecorder) {
				gomock.InOrder(
					m.UpdateScheduler(gomock.Any(), gomock.Any()).Return(nil, nil).Times(1),
					ms.OpenNetworkTopology().Return(io.NopCloser(bytes.NewBuffer(data)), nil).Times(1),
					mt.Send(gomock.Any()).DoAndReturn(
						func(t *trainerv1.TrainRequest) error {
							return nil
						}).AnyTimes(),
				)
			},
			except: func(t *testing.T, a Announcer, err error) {
				assert := assert.New(t)
				assert.NoError(err)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctl := gomock.NewController(t)
			defer ctl.Finish()
			stream := trainerv1mocks.NewMockTrainer_TrainClient(ctl)
			mockManagerClient := managerclientmocks.NewMockV2(ctl)
			mockTrainerClient := trainerclientmocks.NewMockV1(ctl)
			mockStorage := storagemocks.NewMockStorage(ctl)
			tc.mock(stream, tc.data, mockManagerClient.EXPECT(), mockTrainerClient.EXPECT(), mockStorage.EXPECT(), stream.EXPECT())

			a, err := New(tc.config, mockManagerClient, mockStorage, WithTrainerClient(mockTrainerClient))
			if err != nil {
				t.Fatal(err)
			}
			err = a.(*announcer).uploadNetworkTopologyToTrainer(stream)
			tc.except(t, a, err)
		})
	}
}
