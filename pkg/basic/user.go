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

package basic

import (
	"os"
	"os/user"
	"strconv"
	"strings"

	"d7y.io/dragonfly/v2/pkg/util/stringutils"
)

var (
	HomeDir   string
	TmpDir    string
	Username  string
	UserID    int
	UserGroup int
)

func init() {
	u, err := user.Current()
	if err != nil {
		panic(err)
	}

	Username = u.Username
	UserID, _ = strconv.Atoi(u.Uid)
	UserGroup, _ = strconv.Atoi(u.Gid)

	HomeDir = u.HomeDir
	HomeDir = strings.TrimSpace(HomeDir)
	if stringutils.IsBlank(HomeDir) {
		panic("home dir is empty")
	}

	TmpDir = os.TempDir()
	TmpDir = strings.TrimSpace(TmpDir)
	if stringutils.IsBlank(TmpDir) {
		TmpDir = "/tmp"
	}
}
