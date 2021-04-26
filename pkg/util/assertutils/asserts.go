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

package assertutils

import (
	"d7y.io/dragonfly/v2/pkg/util/ifaceutils"
	"github.com/pkg/errors"
)

func AssertTrue(cond bool, message string) error {
	if cond {
		return nil
	} else {
		return errors.New(message)
	}
}

func AssertFalse(cond bool, message string) error {
	if !cond {
		return nil
	} else {
		return errors.New(message)
	}
}

func AssertNil(v interface{}, message string) error {
	if ifaceutils.IsNil(v) {
		return nil
	} else {
		return errors.New(message)
	}
}

func AssertNotNil(v interface{}, message string) error {
	if !ifaceutils.IsNil(v) {
		return nil
	} else {
		return errors.New(message)
	}
}

func PAssert(cond bool, message string) {
	if !cond {
		panic(message)
	}
}
