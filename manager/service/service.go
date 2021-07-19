package service

import (
	"d7y.io/dragonfly/v2/manager/cache"
	"d7y.io/dragonfly/v2/manager/database"
	"d7y.io/dragonfly/v2/manager/model"
	"d7y.io/dragonfly/v2/manager/types"
	"github.com/go-redis/redis/v8"
	"gorm.io/gorm"
)

type REST interface {
	CreateCDNCluster(types.CreateCDNClusterRequest) (*model.CDNCluster, error)
	CreateCDNClusterWithSecurityGroupDomain(types.CreateCDNClusterRequest) (*model.CDNCluster, error)
	DestroyCDNCluster(uint) error
	UpdateCDNCluster(uint, types.UpdateCDNClusterRequest) (*model.CDNCluster, error)
	UpdateCDNClusterWithSecurityGroupDomain(uint, types.UpdateCDNClusterRequest) (*model.CDNCluster, error)
	GetCDNCluster(uint) (*model.CDNCluster, error)
	GetCDNClusters(types.GetCDNClustersQuery) (*[]model.CDNCluster, error)
	CDNClusterTotalCount(types.GetCDNClustersQuery) (int64, error)
	AddCDNToCDNCluster(uint, uint) error
	AddSchedulerClusterToCDNCluster(uint, uint) error

	CreateCDN(types.CreateCDNRequest) (*model.CDN, error)
	DestroyCDN(uint) error
	UpdateCDN(uint, types.UpdateCDNRequest) (*model.CDN, error)
	GetCDN(uint) (*model.CDN, error)
	GetCDNs(types.GetCDNsQuery) (*[]model.CDN, error)
	CDNTotalCount(types.GetCDNsQuery) (int64, error)

	CreateSchedulerCluster(types.CreateSchedulerClusterRequest) (*model.SchedulerCluster, error)
	CreateSchedulerClusterWithSecurityGroupDomain(types.CreateSchedulerClusterRequest) (*model.SchedulerCluster, error)
	DestroySchedulerCluster(uint) error
	UpdateSchedulerCluster(uint, types.UpdateSchedulerClusterRequest) (*model.SchedulerCluster, error)
	UpdateSchedulerClusterWithSecurityGroupDomain(uint, types.UpdateSchedulerClusterRequest) (*model.SchedulerCluster, error)
	GetSchedulerCluster(uint) (*model.SchedulerCluster, error)
	GetSchedulerClusters(types.GetSchedulerClustersQuery) (*[]model.SchedulerCluster, error)
	SchedulerClusterTotalCount(types.GetSchedulerClustersQuery) (int64, error)
	AddSchedulerToSchedulerCluster(uint, uint) error

	CreateScheduler(types.CreateSchedulerRequest) (*model.Scheduler, error)
	DestroyScheduler(uint) error
	UpdateScheduler(uint, types.UpdateSchedulerRequest) (*model.Scheduler, error)
	GetScheduler(uint) (*model.Scheduler, error)
	GetSchedulers(types.GetSchedulersQuery) (*[]model.Scheduler, error)
	SchedulerTotalCount(types.GetSchedulersQuery) (int64, error)

	CreateSecurityGroup(types.CreateSecurityGroupRequest) (*model.SecurityGroup, error)
	DestroySecurityGroup(uint) error
	UpdateSecurityGroup(uint, types.UpdateSecurityGroupRequest) (*model.SecurityGroup, error)
	GetSecurityGroup(uint) (*model.SecurityGroup, error)
	GetSecurityGroups(types.GetSecurityGroupsQuery) (*[]model.SecurityGroup, error)
	SecurityGroupTotalCount(types.GetSecurityGroupsQuery) (int64, error)
	AddSchedulerClusterToSecurityGroup(uint, uint) error
	AddCDNClusterToSecurityGroup(uint, uint) error

	SignIn(json types.SignInRequest) (*model.User, error)
	SignUp(json types.SignUpRequest) (*model.User, error)
}

type rest struct {
	db    *gorm.DB
	rdb   *redis.Client
	cache *cache.Cache
}

// NewREST returns a new REST instence
func NewREST(database *database.Database, cache *cache.Cache) REST {
	return &rest{
		db:    database.DB,
		rdb:   database.RDB,
		cache: cache,
	}
}
