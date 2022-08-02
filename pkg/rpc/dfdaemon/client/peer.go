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

	"google.golang.org/grpc"

	commonv1 "d7y.io/api/pkg/apis/common/v1"
	dfdaemonv1 "d7y.io/api/pkg/apis/dfdaemon/v1"
	schedulerv1 "d7y.io/api/pkg/apis/scheduler/v1"

	"d7y.io/dragonfly/v2/pkg/dfnet"
)

func GetPieceTasks(ctx context.Context,
	dstPeer *schedulerv1.PeerPacket_DestPeer,
	ptr *commonv1.PieceTaskRequest,
	opts ...grpc.CallOption) (*commonv1.PiecePacket, error) {
	netAddr := dfnet.NetAddr{
		Type: dfnet.TCP,
		Addr: fmt.Sprintf("%s:%d", dstPeer.Ip, dstPeer.RpcPort),
	}

	client, err := GetElasticClientByAddrs([]dfnet.NetAddr{netAddr})
	if err != nil {
		return nil, err
	}

	return client.GetPieceTasks(ctx, netAddr, ptr, opts...)
}

func SyncPieceTasks(ctx context.Context,
	destPeer *schedulerv1.PeerPacket_DestPeer,
	ptr *commonv1.PieceTaskRequest,
	opts ...grpc.CallOption) (dfdaemonv1.Daemon_SyncPieceTasksClient, error) {
	netAddr := dfnet.NetAddr{
		Type: dfnet.TCP,
		Addr: fmt.Sprintf("%s:%d", destPeer.Ip, destPeer.RpcPort),
	}

	client, err := GetElasticClientByAddrs([]dfnet.NetAddr{netAddr})
	if err != nil {
		return nil, err
	}

	return client.SyncPieceTasks(ctx, netAddr, ptr, opts...)
}
