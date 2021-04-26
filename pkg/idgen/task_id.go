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

package idgen

import (
	"strings"

	"d7y.io/dragonfly/v2/pkg/rpc/base"
	"d7y.io/dragonfly/v2/pkg/util/digestutils"
	"d7y.io/dragonfly/v2/pkg/util/net/urlutils"
	"d7y.io/dragonfly/v2/pkg/util/stringutils"
)

const (
	TwinsA = "_A"
	TwinsB = "_B"
)

// GenerateTaskID generates a taskId.
// filter is separated by & character.
func GenerateTaskID(url string, filter string, meta *base.UrlMeta, bizID string) string {
	taskIdSource := url
	if filter != "" {
		taskIdSource = urlutils.FilterURLParam(url, strings.Split(filter, "&"))
	}

	var md5String, rangeString string
	if meta != nil {
		md5String = meta.Md5
		rangeString = meta.Range
	}

	if md5String != "" {
		taskIdSource += "|" + md5String
	} else if bizID != "" {
		taskIdSource += "|" + bizID
	}

	if rangeString != "" {
		taskIdSource += "|" + rangeString
	}

	taskIdSource = stringutils.SubString(taskIdSource, len(taskIdSource)-10, len(taskIdSource)) + taskIdSource

	return digestutils.Sha256(taskIdSource)
}

// GenerateTwinsTaskId used A/B testing
func GenerateTwinsTaskID(url string, filter string, meta *base.UrlMeta, bizID, peerID string) string {
	taskID := GenerateTaskID(url, filter, meta, bizID)

	if digestutils.CheckSum(peerID)&1 == 0 {
		taskID += TwinsA
	} else {
		taskID += TwinsB
	}

	return taskID
}
