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

package storedriver

import (
	"fmt"
	"strings"

	"d7y.io/dragonfly/v2/cdnsystem/config"
	"d7y.io/dragonfly/v2/cdnsystem/plugins"
)

// StorageBuilder is a function that creates a new storage plugin instant with the giving conf.
type StorageBuilder func(conf interface{}) (Driver, error)

// Register defines an interface to register a driver with specified name.
// All drivers should call this function to register itself to the driverFactory.
func Register(name string, builder StorageBuilder) {
	name = strings.ToLower(name)
	// plugin builder
	var f plugins.Builder = func(conf interface{}) (plugin plugins.Plugin, e error) {
		return NewStore(name, builder, conf)
	}
	plugins.RegisterPlugin(config.StoragePlugin, name, f)
}

// Get a store from manager with specified name.
func Get(name string) (*Store, error) {
	v := plugins.GetPlugin(config.StoragePlugin, strings.ToLower(name))
	if v == nil {
		return nil, fmt.Errorf("storage: %s not existed", name)
	}
	if store, ok := v.(*Store); ok {
		return store, nil
	}
	return nil, fmt.Errorf("get store error: unknown reason")
}
