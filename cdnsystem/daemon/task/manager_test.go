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

package task

import (
	"context"
	"testing"

	"d7y.io/dragonfly/v2/cdnsystem/config"
	"d7y.io/dragonfly/v2/cdnsystem/daemon/mock"
	"d7y.io/dragonfly/v2/cdnsystem/types"
	"d7y.io/dragonfly/v2/internal/idgen"
	"d7y.io/dragonfly/v2/internal/rpc/base"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/suite"
)

func TestTaskManagerSuite(t *testing.T) {
	suite.Run(t, new(TaskManagerTestSuite))
}

type TaskManagerTestSuite struct {
	tm *Manager
	suite.Suite
}

func (suite *TaskManagerTestSuite) TestRegister() {
	dragonflyURL := "http://dragonfly.io.com?a=a&b=b&c=c"
	taskID := idgen.TaskID(dragonflyURL, "a&b", &base.UrlMeta{Digest: "f1e2488bba4d1267948d9e2f7008571c"}, "dragonfly")
	ctrl := gomock.NewController(suite.T())
	cdnMgr := mock.NewMockCDNMgr(ctrl)
	progressMgr := mock.NewMockSeedProgressMgr(ctrl)
	progressMgr.EXPECT().SetTaskMgr(gomock.Any()).Times(1)
	tm, err := NewManager(config.New(), cdnMgr, progressMgr)
	suite.Nil(err)
	suite.NotNil(tm)
	type args struct {
		ctx context.Context
		req *types.TaskRegisterRequest
	}
	tests := []struct {
		name          string
		args          args
		wantPieceChan <-chan *types.SeedPiece
		wantErr       bool
	}{
		{
			name: "register",
			args: args{
				ctx: context.Background(),
				req: &types.TaskRegisterRequest{
					URL:    dragonflyURL,
					TaskID: taskID,
					Md5:    "f1e2488bba4d1267948d9e2f7008571c",
					Filter: []string{"a", "b"},
					Header: nil,
				},
			},
			wantPieceChan: nil,
			wantErr:       false,
		},
	}
	for _, tt := range tests {
		suite.Run(tt.name, func() {
			//gotPieceChan, err := tm.Register(tt.args.ctx, tt.args.req)
			//
			//if (err != nil) != tt.wantErr {
			//	suite.T().Errorf("Register() error = %v, wantErr %v", err, tt.wantErr)
			//	return
			//}
			//if !reflect.DeepEqual(gotPieceChan, tt.wantPieceChan) {
			//	suite.T().Errorf("Register() gotPieceChan = %v, want %v", gotPieceChan, tt.wantPieceChan)
			//}
		})
	}
}
