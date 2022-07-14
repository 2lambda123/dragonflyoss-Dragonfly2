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

package database

import (
	"fmt"

	"github.com/go-redis/redis/v8"
	"gorm.io/gorm"

	"d7y.io/dragonfly/v2/manager/config"
	"d7y.io/dragonfly/v2/manager/model"
	schedulerconfig "d7y.io/dragonfly/v2/scheduler/config"
)

const (
	// Default name for scheduler cluster.
	DefaultSchedulerClusterName = "scheduler-cluster-1"

	// Default name for seed peer cluster.
	DefaultSeedPeerClusterName = "seed-peer-cluster-1"
)

type Database struct {
	DB  *gorm.DB
	RDB *redis.Client
}

func New(cfg *config.Config) (*Database, error) {
	var (
		db  *gorm.DB
		err error
	)
	switch cfg.Database.Type {
	case config.DatabaseTypeMysql, config.DatabaseTypeMariaDB:
		db, err = newMyqsl(cfg)
		if err != nil {
			return nil, err
		}
	case config.DatabaseTypePostgres:
		db, err = newPostgres(cfg)
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("invalid database type %s", cfg.Database.Type)
	}

	rdb, err := NewRedis(cfg.Database.Redis)
	if err != nil {
		return nil, err
	}

	return &Database{
		DB:  db,
		RDB: rdb,
	}, nil
}

func migrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&model.Job{},
		&model.SeedPeerCluster{},
		&model.SeedPeer{},
		&model.SchedulerCluster{},
		&model.Scheduler{},
		&model.SecurityRule{},
		&model.SecurityGroup{},
		&model.User{},
		&model.Oauth{},
		&model.Config{},
		&model.Application{},
	)
}

func seed(cfg *config.Config, db *gorm.DB) error {
	var schedulerClusterCount int64
	if err := db.Model(model.SchedulerCluster{}).Count(&schedulerClusterCount).Error; err != nil {
		return err
	}

	if schedulerClusterCount <= 0 {
		if err := db.Create(&model.SchedulerCluster{
			Model: model.Model{
				ID: uint(1),
			},
			Name: DefaultSchedulerClusterName,
			Config: map[string]any{
				"filter_parent_limit": schedulerconfig.DefaultSchedulerFilterParentLimit,
			},
			ClientConfig: map[string]any{
				"load_limit":     schedulerconfig.DefaultClientLoadLimit,
				"parallel_count": schedulerconfig.DefaultClientParallelCount,
			},
			Scopes:    map[string]any{},
			IsDefault: true,
		}).Error; err != nil {
			return err
		}
	}

	var seedPeerClusterCount int64
	if err := db.Model(model.SeedPeerCluster{}).Count(&seedPeerClusterCount).Error; err != nil {
		return err
	}

	if seedPeerClusterCount <= 0 {
		if err := db.Create(&model.SeedPeerCluster{
			Model: model.Model{
				ID: uint(1),
			},
			Name: DefaultSeedPeerClusterName,
			Config: map[string]any{
				"load_limit": schedulerconfig.DefaultSeedPeerLoadLimit,
			},
			IsDefault: true,
		}).Error; err != nil {
			return err
		}

		seedPeerCluster := model.SeedPeerCluster{}
		if err := db.First(&seedPeerCluster).Error; err != nil {
			return err
		}

		schedulerCluster := model.SchedulerCluster{}
		if err := db.First(&schedulerCluster).Error; err != nil {
			return err
		}

		if err := db.Model(&seedPeerCluster).Association("SchedulerClusters").Append(&schedulerCluster); err != nil {
			return err
		}
	}

	return nil
}
