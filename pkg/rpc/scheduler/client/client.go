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
	"time"

	"d7y.io/dragonfly/v2/pkg/dfcodes"
	"d7y.io/dragonfly/v2/pkg/dferrors"
	"d7y.io/dragonfly/v2/pkg/idgen"
	"d7y.io/dragonfly/v2/pkg/rpc/manager"
	mgClient "d7y.io/dragonfly/v2/pkg/rpc/manager/client"
	"d7y.io/dragonfly/v2/pkg/util/net/iputils"
	"d7y.io/dragonfly/v2/pkg/util/stringutils"
	"github.com/pkg/errors"
	"google.golang.org/grpc"

	"d7y.io/dragonfly/v2/pkg/basic/dfnet"
	logger "d7y.io/dragonfly/v2/pkg/dflog"
	"d7y.io/dragonfly/v2/pkg/rpc"
	"d7y.io/dragonfly/v2/pkg/rpc/base"
	"d7y.io/dragonfly/v2/pkg/rpc/scheduler"
)

func GetClientByAddr(addrs []dfnet.NetAddr, opts ...grpc.DialOption) (SchedulerClient, error) {
	if len(addrs) == 0 {
		return nil, errors.New("address list of scheduler is empty")
	}
	sc := &schedulerClient{
		rpc.NewConnection(context.Background(), "scheduler-static", addrs, []rpc.ConnOption{
			rpc.WithConnExpireTime(5 * time.Minute),
			rpc.WithDialOption(opts),
		}),
	}
	logger.Infof("scheduler server list: %s", addrs)
	return sc, nil
}

func GetSchedulerByConfigServer(cfgServerAddr string, opts ...grpc.DialOption) (SchedulerClient, error) {
	if stringutils.IsBlank(cfgServerAddr) {
		return nil, fmt.Errorf("config server address is not specified")
	}
	configServer, err := mgClient.NewClient([]dfnet.NetAddr{{
		Type: dfnet.TCP,
		Addr: cfgServerAddr,
	}})
	if err != nil {
		return nil, errors.Wrap(err, "failed to create config server")
	}
	// todo create HostTag
	HostTag := ""
	schedulers, err := configServer.GetSchedulers(context.Background(), &manager.GetSchedulersRequest{
		Ip:       iputils.HostIP,
		HostName: iputils.HostName,
		HostTag:  HostTag,
	})
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get scheduler list from config server")
	}
	var scheds []dfnet.NetAddr
	for i := range schedulers.Addrs {
		scheds = append(scheds, dfnet.NetAddr{
			Type: dfnet.TCP,
			Addr: schedulers.Addrs[i],
		})
	}
	logger.Infof("successfully get scheduler list: %s", scheds)
	sc := &schedulerClient{
		Connection: rpc.NewConnection(context.Background(), "scheduler-dynamic", scheds, []rpc.ConnOption{
			rpc.WithConnExpireTime(5 * time.Minute),
			rpc.WithDialOption(opts),
		}),
	}
	logger.Infof("scheduler server list: %s", scheds)
	return sc, nil
}

// see scheduler.SchedulerClient
type SchedulerClient interface {
	RegisterPeerTask(context.Context, *scheduler.PeerTaskRequest, ...grpc.CallOption) (*scheduler.RegisterResult, error)
	// IsMigrating of ptr will be set to true
	ReportPieceResult(context.Context, string, *scheduler.PeerTaskRequest, ...grpc.CallOption) (PeerPacketStream, error)

	ReportPeerResult(context.Context, *scheduler.PeerResult, ...grpc.CallOption) error

	LeaveTask(context.Context, *scheduler.PeerTarget, ...grpc.CallOption) error

	Close() error
}

type schedulerClient struct {
	*rpc.Connection
}

func (sc *schedulerClient) getSchedulerClient(key string, stick bool) (scheduler.SchedulerClient, string, error) {
	clientConn, err := sc.Connection.GetClientConn(key, stick)
	if err != nil {
		return nil, "", err
	}
	return scheduler.NewSchedulerClient(clientConn), clientConn.Target(), nil
}

func (sc *schedulerClient) RegisterPeerTask(ctx context.Context, ptr *scheduler.PeerTaskRequest, opts ...grpc.CallOption) (rr *scheduler.RegisterResult,
	err error) {
	return sc.doRegisterPeerTask(ctx, ptr, []string{}, opts)
}

func (sc *schedulerClient) doRegisterPeerTask(ctx context.Context, ptr *scheduler.PeerTaskRequest, exclusiveNodes []string,
	opts []grpc.CallOption) (rr *scheduler.RegisterResult, err error) {
	var (
		taskID        string
		suc           bool
		code          base.Code
		schedulerNode string
		res           interface{}
	)
	key := idgen.TaskID(ptr.Url, ptr.Filter, ptr.UrlMata, ptr.BizId)
	logger.WithPeerID(ptr.PeerId).Infof("generate hash key taskId: %s and start to register peer task for peer_id(%s) url(%s)", key, ptr.PeerId, ptr.Url)
	if res, err = rpc.ExecuteWithRetry(func() (interface{}, error) {
		var client scheduler.SchedulerClient
		client, schedulerNode, err = sc.getSchedulerClient(key, false)
		if err != nil {
			code = dfcodes.ServerUnavailable
			return nil, err
		}
		return client.RegisterPeerTask(ctx, ptr, opts...)
	}, 0.5, 5.0, 5, nil); err == nil {
		rr = res.(*scheduler.RegisterResult)
		taskID = rr.TaskId
		suc = true
		code = dfcodes.Success
		if taskID != key {
			logger.WithPeerID(ptr.PeerId).Warnf("register peer task correct taskId from %s to %s", key, taskID)
			sc.Connection.CorrectKey2NodeRelation(key, taskID)
		}
	} else {
		if de, ok := err.(*dferrors.DfError); ok {
			code = de.Code
		}
	}
	logger.With("peerId", ptr.PeerId, "errMsg", err).
		Infof("register peer task result:%t[%d] for taskId:%s,url:%s,peerIp:%s,securityDomain:%s,idc:%s,scheduler:%s",
			suc, int32(code), taskID, ptr.Url, ptr.PeerHost.Ip, ptr.PeerHost.SecurityDomain, ptr.PeerHost.Idc, schedulerNode)

	if err != nil {
		var preNode string
		if preNode, err = sc.TryMigrate(key, err, exclusiveNodes); err == nil {
			exclusiveNodes = append(exclusiveNodes, preNode)
			return sc.doRegisterPeerTask(ctx, ptr, exclusiveNodes, opts)
		}
	}
	return
}

func (sc *schedulerClient) ReportPieceResult(ctx context.Context, taskID string, ptr *scheduler.PeerTaskRequest, opts ...grpc.CallOption) (PeerPacketStream, error) {
	pps, err := newPeerPacketStream(ctx, sc, taskID, ptr, opts)

	logger.With("peerId", ptr.PeerId, "errMsg", err).Infof("start to report piece result for taskID:%s", taskID)

	// trigger scheduling
	pps.Send(scheduler.NewZeroPieceResult(taskID, ptr.PeerId))
	return pps, err
}

func (sc *schedulerClient) ReportPeerResult(ctx context.Context, pr *scheduler.PeerResult, opts ...grpc.CallOption) error {
	return sc.doReportPeerResult(ctx, pr, []string{}, opts)
}

func (sc *schedulerClient) doReportPeerResult(ctx context.Context, pr *scheduler.PeerResult, exclusiveNodes []string, opts []grpc.CallOption) (err error) {
	var (
		schedulerNode string
		suc           bool
		code          base.Code
	)

	_, err = rpc.ExecuteWithRetry(func() (interface{}, error) {
		var client scheduler.SchedulerClient
		client, schedulerNode, err = sc.getSchedulerClient(pr.TaskId, true)
		if err != nil {
			code = dfcodes.ServerUnavailable
			return nil, err
		}
		return client.ReportPeerResult(ctx, pr, opts...)
	}, 0.5, 5.0, 5, nil)
	if err == nil {
		suc = true
		code = dfcodes.Success
	}

	logger.With("peerId", pr.PeerId, "errMsg", err).
		Infof("report peer result:%t[%d], peer task down result:%t[%d] for taskId:%s,url:%s,scheduler:%s,length:%d,traffic:%d,cost:%d", suc, int32(code),
			pr.Success, int32(pr.Code), pr.TaskId, pr.Url, schedulerNode, pr.ContentLength, pr.Traffic, pr.Cost)

	if err != nil {
		var preNode string
		if preNode, err = sc.TryMigrate(pr.TaskId, err, exclusiveNodes); err == nil {
			exclusiveNodes = append(exclusiveNodes, preNode)
			return sc.doReportPeerResult(ctx, pr, exclusiveNodes, opts)
		}
	}

	return
}

func (sc *schedulerClient) LeaveTask(ctx context.Context, pt *scheduler.PeerTarget, opts ...grpc.CallOption) (err error) {
	var (
		schedulerNode string
		suc           bool
	)
	defer func() {
		logger.With("peerId", pt.PeerId, "errMsg", err).Infof("leave from task result:%t for taskId:%s,scheduler:%s", suc, pt.TaskId, schedulerNode)
	}()

	_, err = rpc.ExecuteWithRetry(func() (interface{}, error) {
		var client scheduler.SchedulerClient
		client, schedulerNode, err = sc.getSchedulerClient(pt.TaskId, true)
		if err != nil {
			return nil, err
		}
		return client.LeaveTask(ctx, pt, opts...)
	}, 0.5, 5.0, 3, nil)
	if err == nil {
		suc = true
	}

	return
}

var _ SchedulerClient = (*schedulerClient)(nil)
