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

	"github.com/pkg/errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"d7y.io/dragonfly/v2/internal/dferrors"
	logger "d7y.io/dragonfly/v2/internal/dflog"
	"d7y.io/dragonfly/v2/pkg/rpc"
	"d7y.io/dragonfly/v2/pkg/rpc/dfdaemon"
)

type DownResultStream struct {
	dc      *daemonClient
	ctx     context.Context
	hashKey string
	req     *dfdaemon.DownRequest
	opts    []grpc.CallOption
	// stream for one client
	stream        dfdaemon.Daemon_DownloadClient
	failedServers []string
	rpc.RetryMeta
}

func newDownResultStream(ctx context.Context, dc *daemonClient, hashKey string, req *dfdaemon.DownRequest, opts []grpc.CallOption) (*DownResultStream, error) {
	drs := &DownResultStream{
		dc:      dc,
		ctx:     ctx,
		hashKey: hashKey,
		req:     req,
		opts:    opts,

		RetryMeta: rpc.RetryMeta{
			MaxAttempts: 3,
			MaxBackOff:  2.0,
			InitBackoff: 0.2,
		},
	}

	if err := drs.initStream(); err != nil {
		return nil, err
	}
	return drs, nil
}

func (drs *DownResultStream) initStream() error {
	client, err := drs.dc.getDaemonClient()
	if err != nil {
		return err
	}
	stream, err := client.Download(drs.ctx, drs.req, drs.opts...)
	if err != nil {
		if errors.Cause(err) == dferrors.ErrNoCandidateNode {
			return errors.Wrapf(err, "get grpc server instance failed")
		}
		logger.WithTaskID(drs.hashKey).Infof("initStream: invoke daemon Download failed: %v", err)
		return drs.replaceClient(err)
	}
	drs.stream = stream
	drs.StreamTimes = 1
	return nil
}

func (drs *DownResultStream) Recv() (dr *dfdaemon.DownResult, err error) {
	defer func() {
		if dr != nil {
			if dr.TaskId != drs.hashKey {
				logger.WithTaskAndPeerID(dr.TaskId, dr.PeerId).Warnf("down result stream correct taskId from %s to %s", drs.hashKey, dr.TaskId)
				drs.hashKey = dr.TaskId
			}
		}
	}()
	return drs.stream.Recv()
}

func (drs *DownResultStream) retryRecv(cause error) (*dfdaemon.DownResult, error) {
	if status.Code(cause) == codes.DeadlineExceeded || status.Code(cause) == codes.Canceled {
		return nil, cause
	}

	if err := drs.replaceStream(cause); err != nil {
		return nil, err
	}

	return drs.Recv()
}

func (drs *DownResultStream) replaceStream(cause error) error {
	if drs.StreamTimes >= drs.MaxAttempts {
		logger.WithTaskID(drs.hashKey).Info("replace stream reach max attempt")
		return cause
	}
	client, err := drs.dc.getDaemonClient()
	if err != nil {
		return err
	}
	stream, err := client.Download(drs.ctx, drs.req, drs.opts...)
	if err != nil {
		logger.WithTaskID(drs.hashKey).Infof("replaceStream: invoke daemon Download failed: %v", err)
		return drs.replaceClient(cause)
	}
	drs.stream = stream
	drs.StreamTimes++
	return nil
}

func (drs *DownResultStream) replaceClient(cause error) error {
	client, err := drs.dc.getDaemonClient()
	if err != nil {
		return err
	}
	stream, err := client.Download(drs.ctx, drs.req, drs.opts...)
	if err != nil {
		logger.WithTaskID(drs.hashKey).Infof("replaceClient: invoke daemon Download failed: %v", err)
		return drs.replaceClient(cause)
	}
	drs.stream = stream
	drs.StreamTimes = 1
	return nil
}
