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

//go:generate mockgen -destination mocks/client_mock.go -source client.go -package mocks

package client

import (
	"context"
	"fmt"
	"sync"
	"time"

	"google.golang.org/grpc"

	cdnsystemv1 "d7y.io/api/pkg/apis/cdnsystem/v1"
	commonv1 "d7y.io/api/pkg/apis/common/v1"

	logger "d7y.io/dragonfly/v2/internal/dflog"
	"d7y.io/dragonfly/v2/pkg/dfnet"
	"d7y.io/dragonfly/v2/pkg/rpc"
)

func GetClientByAddr(addrs []dfnet.NetAddr, opts ...grpc.DialOption) CdnClient {
	return &cdnClient{
		rpc.NewConnection(context.Background(), "cdn", addrs, []rpc.ConnOption{
			rpc.WithConnExpireTime(60 * time.Second),
			rpc.WithDialOption(opts),
		}),
	}
}

var once sync.Once
var elasticCdnClient *cdnClient

func GetElasticClientByAddrs(addrs []dfnet.NetAddr, opts ...grpc.DialOption) (CdnClient, error) {
	once.Do(func() {
		elasticCdnClient = &cdnClient{
			rpc.NewConnection(context.Background(), "cdn-elastic", make([]dfnet.NetAddr, 0), []rpc.ConnOption{
				rpc.WithConnExpireTime(60 * time.Second),
				rpc.WithDialOption(opts),
			}),
		}
	})
	err := elasticCdnClient.Connection.AddServerNodes(addrs)
	if err != nil {
		return nil, err
	}
	return elasticCdnClient, nil
}

// CdnClient see cdnsystemv1.CdnClient
type CdnClient interface {
	ObtainSeeds(ctx context.Context, sr *cdnsystemv1.SeedRequest, opts ...grpc.CallOption) (*PieceSeedStream, error)

	GetPieceTasks(ctx context.Context, addr dfnet.NetAddr, req *commonv1.PieceTaskRequest, opts ...grpc.CallOption) (*commonv1.PiecePacket, error)

	SyncPieceTasks(ctx context.Context, addr dfnet.NetAddr, ptr *commonv1.PieceTaskRequest, opts ...grpc.CallOption) (cdnsystemv1.Seeder_SyncPieceTasksClient, error)

	UpdateState(addrs []dfnet.NetAddr)

	Close() error
}

type cdnClient struct {
	*rpc.Connection
}

var _ CdnClient = (*cdnClient)(nil)

func (cc *cdnClient) getCdnClient(key string, stick bool) (cdnsystemv1.SeederClient, string, error) {
	clientConn, err := cc.Connection.GetClientConn(key, stick)
	if err != nil {
		return nil, "", fmt.Errorf("get ClientConn for hashKey %s: %w", key, err)
	}
	return cdnsystemv1.NewSeederClient(clientConn), clientConn.Target(), nil
}

func (cc *cdnClient) getSeederClientWithTarget(target string) (cdnsystemv1.SeederClient, error) {
	conn, err := cc.Connection.GetClientConnByTarget(target)
	if err != nil {
		return nil, err
	}
	return cdnsystemv1.NewSeederClient(conn), nil
}

func (cc *cdnClient) ObtainSeeds(ctx context.Context, sr *cdnsystemv1.SeedRequest, opts ...grpc.CallOption) (*PieceSeedStream, error) {
	return newPieceSeedStream(ctx, cc, sr.TaskId, sr, opts)
}

func (cc *cdnClient) GetPieceTasks(ctx context.Context, addr dfnet.NetAddr, req *commonv1.PieceTaskRequest, opts ...grpc.CallOption) (*commonv1.PiecePacket, error) {
	client, err := cc.getSeederClientWithTarget(addr.GetEndpoint())
	if err != nil {
		return nil, err
	}
	return client.GetPieceTasks(ctx, req, opts...)
}

func (cc *cdnClient) SyncPieceTasks(ctx context.Context, addr dfnet.NetAddr, req *commonv1.PieceTaskRequest, opts ...grpc.CallOption) (cdnsystemv1.Seeder_SyncPieceTasksClient, error) {
	client, err := cc.getSeederClientWithTarget(addr.GetEndpoint())
	if err != nil {
		return nil, err
	}
	syncClient, err := client.SyncPieceTasks(ctx, opts...)
	if err != nil {
		logger.WithTaskID(req.TaskId).Infof("SyncPieceTasks: invoke cdn node %s SyncPieceTasks failed: %v", addr.GetEndpoint(), err)
		return nil, err
	}
	return syncClient, syncClient.Send(req)
}
