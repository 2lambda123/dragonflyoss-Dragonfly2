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

	logger "d7y.io/dragonfly/v2/pkg/dflog"
	"d7y.io/dragonfly/v2/pkg/rpc/manager"
	mgClient "d7y.io/dragonfly/v2/pkg/rpc/manager/client"
	"d7y.io/dragonfly/v2/pkg/util/net/iputils"
)

type DynHostsMgr struct {
	managerCli mgClient.ManagerClient
}

func NewCdnHostsMgr(cfgServer mgClient.ManagerClient) *DynHostsMgr {
	return &DynHostsMgr{managerCli: cfgServer}
}

func (md *DynHostsMgr) Get() (interface{}, error) {
	scConfig, err := md.managerCli.GetSchedulerClusterConfig(context.Background(), &manager.GetClusterConfigRequest{
		HostName: iputils.HostName,
		Type:     manager.ResourceType_Scheduler,
	})
	logger.Debugf("scheduler config from manager is: %v", scConfig)
	if err != nil {
		return nil, err
	}
	return scConfig.CdnHosts, nil
}
