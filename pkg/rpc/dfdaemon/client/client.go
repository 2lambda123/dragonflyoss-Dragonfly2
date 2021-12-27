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

package client

import (
	"context"
	"fmt"

	"github.com/golang/protobuf/ptypes/empty"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	"google.golang.org/grpc"

	logger "d7y.io/dragonfly/v2/internal/dflog"
	"d7y.io/dragonfly/v2/internal/dfnet"
	"d7y.io/dragonfly/v2/pkg/idgen"
	"d7y.io/dragonfly/v2/pkg/rpc"
	"d7y.io/dragonfly/v2/pkg/rpc/base"
	"d7y.io/dragonfly/v2/pkg/rpc/dfdaemon"
)

var _ DaemonClient = (*daemonClient)(nil)

func GetClientByAddr(addrs []dfnet.NetAddr, opts ...grpc.DialOption) (DaemonClient, error) {
	if len(addrs) == 0 {
		return nil, errors.New("address list of dfdaemon is empty")
	}
	resolver := rpc.NewD7yResolver(rpc.DaemonScheme, addrs)

	dialOpts := append(append(append(
		rpc.DefaultClientOpts,
		grpc.WithDefaultServiceConfig(fmt.Sprintf(`{"loadBalancingPolicy": "%s"}`, rpc.D7yBalancerPolicy))),
		grpc.WithResolvers(resolver)),
		opts...)

	ctx, cancel := context.WithCancel(context.Background())

	conn, err := grpc.DialContext(
		ctx,
		// "dfdaemon.Daemon" is the dfdaemon._Daemon_serviceDesc.ServiceName
		fmt.Sprintf("%s:///%s", rpc.DaemonScheme, "dfdaemon.Daemon"),
		dialOpts...,
	)
	if err != nil {
		cancel()
		return nil, err
	}

	cc := &daemonClient{
		ctx:          ctx,
		cancel:       cancel,
		daemonClient: dfdaemon.NewDaemonClient(conn),
		conn:         conn,
		resolver:     resolver,
	}
	return cc, nil
}

func GetElasticClientByAddr(addr dfnet.NetAddr, opts ...grpc.DialOption) (DaemonClient, error) {
	ctx, cancel := context.WithCancel(context.Background())
	conn, err := getClientByAddr(ctx, addr, opts...)
	if err != nil {
		cancel()
		return nil, err
	}

	return &daemonClient{
		ctx:          ctx,
		cancel:       cancel,
		daemonClient: dfdaemon.NewDaemonClient(conn),
		conn:         conn,
	}, nil
}

func getClientByAddr(ctx context.Context, addr dfnet.NetAddr, opts ...grpc.DialOption) (*grpc.ClientConn, error) {
	dialOpts := append(append(rpc.DefaultClientOpts, grpc.WithDisableServiceConfig()), opts...)
	return grpc.DialContext(ctx, addr.GetEndpoint(), dialOpts...)
}

// DaemonClient see dfdaemon.DaemonClient
type DaemonClient interface {
	Download(ctx context.Context, req *dfdaemon.DownRequest, opts ...grpc.CallOption) (*DownResultStream, error)

	GetPieceTasks(ctx context.Context, addr dfnet.NetAddr, ptr *base.PieceTaskRequest, opts ...grpc.CallOption) (*base.PiecePacket, error)

	CheckHealth(ctx context.Context, target dfnet.NetAddr, opts ...grpc.CallOption) error

	Close() error
}

type daemonClient struct {
	ctx          context.Context
	cancel       context.CancelFunc
	daemonClient dfdaemon.DaemonClient
	conn         *grpc.ClientConn
	resolver     *rpc.D7yResolver
}

func (dc *daemonClient) getDaemonClient() (dfdaemon.DaemonClient, error) {
	return dc.daemonClient, nil
}

func (dc *daemonClient) getDaemonClientByAddr(addr dfnet.NetAddr) (dfdaemon.DaemonClient, error) {
	conn, err := getClientByAddr(dc.ctx, addr)
	if err != nil {
		return nil, err
	}
	return dfdaemon.NewDaemonClient(conn), nil
}

func (dc *daemonClient) Download(ctx context.Context, req *dfdaemon.DownRequest, opts ...grpc.CallOption) (*DownResultStream, error) {
	req.Uuid = uuid.New().String()
	// generate taskID
	taskID := idgen.TaskID(req.Url, req.UrlMeta)
	return newDownResultStream(context.WithValue(ctx, rpc.PickKey{}, &rpc.PickReq{Key: taskID, Attempt: 1}), dc, taskID, req, opts)
}

func (dc *daemonClient) GetPieceTasks(ctx context.Context, target dfnet.NetAddr, ptr *base.PieceTaskRequest, opts ...grpc.CallOption) (*base.PiecePacket,
	error) {
	client, err := dc.getDaemonClientByAddr(target)
	if err != nil {
		return nil, err
	}
	res, err := client.GetPieceTasks(ctx, ptr, opts...)
	if err != nil {
		logger.WithTaskID(ptr.TaskId).Infof("GetPieceTasks: invoke daemon node %s GetPieceTasks failed: %v", target, err)
		return nil, err
	}
	return res, nil
}

func (dc *daemonClient) CheckHealth(ctx context.Context, target dfnet.NetAddr, opts ...grpc.CallOption) (err error) {
	client, err := dc.getDaemonClientByAddr(target)
	if err != nil {
		return err
	}
	_, err = client.CheckHealth(ctx, new(empty.Empty), opts...)
	if err != nil {
		logger.Infof("CheckHealth: invoke daemon node %s CheckHealth failed: %v", target, err)
		return
	}
	return
}

func (dc *daemonClient) Close() error {
	dc.cancel()
	return nil
}
