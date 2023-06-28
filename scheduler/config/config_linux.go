// +build linux

package config

import "runtime"

var (
	SchedulerConfigPath = "/etc/dragonfly/scheduler.yaml"
)

var config = Config{
	Server: ServerConfig{
		Port: 8002,
	},
	Worker: SchedulerWorkerConfig{
		WorkerNum:         runtime.GOMAXPROCS(0),
		WorkerJobPoolSize: 10000,
		SenderNum:         10,
		SenderJobPoolSize: 10000,
	},
	Scheduler: SchedulerConfig{
		ABTest: false,
	},
	CDN: CDNConfig{
		Servers: []CDNServerConfig{
			{
				Name:         "cdn",
				IP:           "127.0.0.1",
				RpcPort:      8003,
				DownloadPort: 8001,
			},
		},
	},
	GC: GCConfig{
		TaskDelay:     3600 * 1000,
		PeerTaskDelay: 3600 * 1000,
	},
}
