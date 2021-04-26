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

package main

import (
	"d7y.io/dragonfly/v2/pkg/basic/dfnet"
	"d7y.io/dragonfly/v2/pkg/dflog/logcore"
	"d7y.io/dragonfly/v2/pkg/rpc/base"
	_ "d7y.io/dragonfly/v2/pkg/rpc/scheduler/server"
	"sync"
)

import (
	"context"
	"fmt"

	"d7y.io/dragonfly/v2/pkg/rpc/cdnsystem"
	"d7y.io/dragonfly/v2/pkg/rpc/cdnsystem/client"
)

func main() {
	logcore.InitCdnSystem(true)
	c, err := client.GetClientByAddr([]dfnet.NetAddr{
		{
			Type: dfnet.TCP,
			Addr: "127.0.0.1:8003",
		},
	})
	if err != nil {
		panic(err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 1; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			psc, _ := c.ObtainSeeds(context.TODO(), &cdnsystem.SeedRequest{
				//TaskId: "test1",
				//Url:    "oss://alimonitor-monitor/serverdd.xml",
				//UrlMeta: &base.UrlMeta{
				//	Header: map[string]string{
				//		"endpoint":        "http://oss-cn-hangzhou-zmf.aliyuncs.com",
				//		"accessKeyID":     "RX8yefyaWDWf15SV",
				//		"accessKeySecret": "hPExQDzDPHepZA7W6N5U7skJqLZGhy",
				//	},
				//},
				TaskId: "test2",
				Url: "https://desktop.docker.com/mac/stable/amd64/Docker.dmg",
				Filter: "",
			})
			for {
				piece, err := psc.Recv()
				if err != nil {
					fmt.Println(err)
					break
				}
				fmt.Println(piece)
			}
		}()
	}
	wg.Wait()
	fmt.Println("client finish")
}

func main2() {
	c, err := client.GetClientByAddr([]dfnet.NetAddr{
		{
			Type: dfnet.TCP,
			Addr: "localhost:8003",
		},
	})
	if err != nil {
		panic(err)
	}

	psc, err := c.GetPieceTasks(context.TODO(), dfnet.NetAddr{
		Type: dfnet.TCP,
		Addr: "localhost:8003",
	}, &base.PieceTaskRequest{
		TaskId:   "test",
		SrcPid:   "11.11.11.11",
		StartNum: 1,
		Limit:    4,
	})
	if err != nil {
		fmt.Printf("%v", err)
	}

	fmt.Printf("client finish:%v", psc)
}
