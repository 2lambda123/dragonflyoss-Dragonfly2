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

package config

import (
	"net"
	"time"

	"d7y.io/dragonfly/v2/pkg/net/ip"
)

const (
	// DefaultSeedPeerConcurrentUploadLimit is default number for seed peer concurrent upload limit.
	DefaultSeedPeerConcurrentUploadLimit = 300

	// DefaultPeerConcurrentUploadLimit is default number for peer concurrent upload limit.
	DefaultPeerConcurrentUploadLimit = 50

	// DefaultPeerConcurrentPieceCount is default number for pieces to concurrent downloading.
	DefaultPeerConcurrentPieceCount = 4

	// DefaultSchedulerFilterParentLimit is default limit the number for filter traversals.
	DefaultSchedulerFilterParentLimit = 4

	// DefaultSchedulerFilterParentRangeLimit is default limit the range for filter traversals.
	DefaultSchedulerFilterParentRangeLimit = 40
)

const (
	// DefaultServerPort is default port for server.
	DefaultServerPort = 8002
)

const (
	// DefaultSchedulerAlgorithm is default algorithm for scheduler.
	DefaultSchedulerAlgorithm = "default"

	// DefaultSchedulerBackToSourceCount is default back-to-source count for scheduler.
	DefaultSchedulerBackToSourceCount = 3

	// DefaultSchedulerRetryBackToSourceLimit is default retry back-to-source limit for scheduler.
	DefaultSchedulerRetryBackToSourceLimit = 5

	// DefaultSchedulerRetryLimit is default retry limit for scheduler.
	DefaultSchedulerRetryLimit = 10

	// DefaultSchedulerRetryInterval is default retry interval for scheduler.
	DefaultSchedulerRetryInterval = 50 * time.Millisecond

	// DefaultSchedulerPieceDownloadTimeout is default timeout of downloading piece.
	DefaultSchedulerPieceDownloadTimeout = 30 * time.Minute

	// DefaultSchedulerPeerGCInterval is default interval for peer gc.
	DefaultSchedulerPeerGCInterval = 10 * time.Second

	// DefaultSchedulerPeerTTL is default ttl for peer.
	DefaultSchedulerPeerTTL = 24 * time.Hour

	// DefaultSchedulerTaskGCInterval is default interval for task gc.
	DefaultSchedulerTaskGCInterval = 30 * time.Minute

	// DefaultSchedulerHostGCInterval is default interval for host gc.
	DefaultSchedulerHostGCInterval = 6 * time.Hour

	// DefaultSchedulerHostTTL is default ttl for host.
	DefaultSchedulerHostTTL = 1 * time.Hour

	// DefaultRefreshModelInterval is model refresh interval.
	DefaultRefreshModelInterval = 168 * time.Hour

	// DefaultCPU is default cpu usage.
	DefaultCPU = 1
)

const (
	// DefaultDynConfigRefreshInterval is default refresh interval for dynamic configuration.
	DefaultDynConfigRefreshInterval = 10 * time.Second
)

const (
	// DefaultManagerSchedulerClusterID is default id for scheduler cluster.
	DefaultManagerSchedulerClusterID = 1

	// DefaultManagerKeepAliveInterval is default interval for keepalive.
	DefaultManagerKeepAliveInterval = 5 * time.Second
)

const (
	// DefaultJobGlobalWorkerNum is default global worker number for job.
	DefaultJobGlobalWorkerNum = 500

	// DefaultJobSchedulerWorkerNum is default scheduler worker number for job.
	DefaultJobSchedulerWorkerNum = 500

	// DefaultJobGlobalWorkerNum is default local worker number for job.
	DefaultJobLocalWorkerNum = 1000

	// DefaultJobRedisBrokerDB is default db for redis broker.
	DefaultJobRedisBrokerDB = 1

	// DefaultJobRedisBackendDB is default db for redis backend.
	DefaultJobRedisBackendDB = 2
)

const (
	// DefaultMetricsAddr is default address for metrics server.
	DefaultMetricsAddr = ":8000"
)

var (
	// DefaultCertIPAddresses is default ip addresses of certificate.
	DefaultCertIPAddresses = []net.IP{ip.IPv4, ip.IPv6}

	// DefaultCertDNSNames is default dns names of certificate.
	DefaultCertDNSNames = []string{"dragonfly-scheduler", "dragonfly-scheduler.dragonfly-system.svc", "dragonfly-scheduler.dragonfly-system.svc.cluster.local"}

	// DefaultCertValidityPeriod is default validity period of certificate.
	DefaultCertValidityPeriod = 180 * 24 * time.Hour
)

var (
	// DefaultNetworkEnableIPv6 is default value of enableIPv6.
	DefaultNetworkEnableIPv6 = false
)

const (
	// DefaultStorageMaxSize is the default maximum size of record file.
	DefaultStorageMaxSize = 100

	// DefaultStorageMaxBackups is the default maximum count of backup.
	DefaultStorageMaxBackups = 10

	// DefaultStorageBufferSize is the default size of buffer container.
	DefaultStorageBufferSize = 100
)

const (
	// TODO(XZ): The default setting needs to be changed after testing.
	// DefaultNetworkTopologySyncInterval is the default interval of synchronizing network topology between schedulers.
	DefaultNetworkTopologySyncInterval = 30 * time.Second

	// TODO(XZ): The default setting needs to be changed after testing.
	// DefaultNetworkTopologyCollectInterval is the default interval of collecting network topology.
	DefaultNetworkTopologyCollectInterval = 60 * time.Second

	// DefaultProbeQueueLength is the default length of probe queue in directed graph.
	DefaultProbeQueueLength = 5

	// TODO(XZ): The default setting needs to be changed after testing.
	// DefaultProbeSyncInterval is the default interval of synchronizing host's probes.
	DefaultProbeSyncInterval = 30 * time.Second

	// TODO(XZ): The default setting needs to be changed after testing.
	// DefaultProbeSyncCount is the default number of probing hosts.
	DefaultProbeSyncCount = 50
)

var (
	// TODO(fyx): The default setting needs to be changed after testing.
	// DefaultTrainerIP is the default ip for trainer
	DefaultTrainerIP = net.ParseIP("177.7.0.1")

	// TODO(fyx): The default setting needs to be changed after testing.
	// DefaultTrainerPort is the default port for trainer
	DefaultTrainerPort = 8509
)

const (
	// TODO(fyx): The default setting needs to be changed after testing.
	// DefaultRefreshInterval is the default interval for refreshing model.
	DefaultRefreshInterval = 3 * 24 * time.Hour

	// TODO(fyx): The default setting needs to be changed after testing.
	// DefaultNetworkRcordLocalPath is the default network record storage path.
	DefaultNetworkRcordLocalPath = "./networkRecord/"

	// TODO(fyx): The default setting needs to be changed after testing.
	// DefaultNetworkRcordMaxSize is the default total maximum size in megabytes of network records in one training process
	DefaultNetworkRcordMaxSize = 1024

	// TODO(fyx): The default setting needs to be changed after testing.
	// DefaultNetworkRcordUnitSize is unit size in megabytes of the network record.
	DefaultNetworkRcordUnitSize = 100

	// TODO(fyx): The default setting needs to be changed after testing.
	// DefaultHistoricalRecordLocalPath is the default historical record storage path.
	DefaultHistoricalRecordLocalPath = "./historicalRecord/"

	// TODO(fyx): The default setting needs to be changed after testing.
	// DefaultNetworkRcordMaxSize is the default total maximum size in megabytes of historical records in one training process
	DefaultHistoricalRecordMaxSize = 1024

	// TODO(fyx): The default setting needs to be changed after testing.
	// DefaultHistoricalRecordUnitSize is unit size in megabytes of the historical record.
	DefaultHistoricalRecordUnitSize = 100
)
