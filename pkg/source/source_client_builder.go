/*
 *     Copyright 2022 The Dragonfly Authors
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

package source

import (
	"fmt"

	"gopkg.in/yaml.v3"

	commonv1 "d7y.io/api/pkg/apis/common/v1"
)

var (
	resourceClientBuilder = map[string]ResourceClientBuilder{}
	resourceClientOptions = map[string]interface{}{}
	resourceAuthInjector  = map[string]AuthInfoInjector{}
)

// ResourceClientBuilder is used to build resource client with custom option
type ResourceClientBuilder interface {
	// Build return the target resource with custom option
	Build(optionYaml []byte) (resourceClient ResourceClient, adaptor RequestAdapter, hooks []Hook, err error)
}

// AuthInfoInjector will inject auth information for target url and metadata, eg: fetch docker config for different users
type AuthInfoInjector interface {
	Inject(url string, urlMeta *commonv1.UrlMeta) error
}

// RegisterOption is used for extra options when registering, like mark target scheme protocol should inject auth information
type RegisterOption func(scheme string)

// RegisterBuilder register ResourceClientBuilder into global resourceClientBuilder, the InitSourceClients will use it.
func RegisterBuilder(scheme string, builder ResourceClientBuilder, opts ...RegisterOption) {
	if _, ok := resourceClientBuilder[scheme]; ok {
		panic(fmt.Sprintf("duplicate ResourceClientBuilder: %s", scheme))
	}
	resourceClientBuilder[scheme] = builder

	for _, opt := range opts {
		opt(scheme)
	}
}

func WithAuthInfoInjector(inj AuthInfoInjector) RegisterOption {
	return func(scheme string) {
		resourceAuthInjector[scheme] = inj
	}
}

func ShouldInjectAuthInfo(scheme string) (AuthInfoInjector, bool) {
	inj, ok := resourceAuthInjector[scheme]
	return inj, ok
}

func UnRegisterBuilder(scheme string) {
	if _, ok := resourceClientBuilder[scheme]; !ok {
		panic(fmt.Sprintf("scheme ResourceClientBuilder %s not found", scheme))
	}
	delete(resourceClientBuilder, scheme)
}

// InitSourceClients will initialize all resource clients which registered by RegisterBuilder.
func InitSourceClients(opts map[string]interface{}) error {
	// save options for resource plugin
	resourceClientOptions = opts

	for scheme, builder := range resourceClientBuilder {
		var (
			opt []byte
			err error
		)
		if data, ok := resourceClientOptions[scheme]; ok {
			opt, err = yaml.Marshal(data)
			if err != nil {
				return err
			}
		}
		resourceClient, adaptor, hooks, err := builder.Build(opt)
		if err != nil {
			return fmt.Errorf("build resource client %s error: %s, options: %s", scheme, err, string(opt))
		}
		err = _defaultManager.Register(scheme, resourceClient, adaptor, hooks...)
		if err != nil {
			return fmt.Errorf("register resource client %s error: %s, options: %s", scheme, err, string(opt))
		}
	}
	return nil
}

type plainResourceClientBuilder struct {
	build func(optionYaml []byte) (resourceClient ResourceClient, adaptor RequestAdapter, hooks []Hook, err error)
}

func (b *plainResourceClientBuilder) Build(optionYaml []byte) (resourceClient ResourceClient, adaptor RequestAdapter, hooks []Hook, err error) {
	return b.build(optionYaml)
}

func NewPlainResourceClientBuilder(
	build func(optionYaml []byte) (resourceClient ResourceClient, adaptor RequestAdapter, hooks []Hook, err error)) ResourceClientBuilder {
	return &plainResourceClientBuilder{build: build}
}

type plainAuthInfoInjector struct {
	inject func(url string, urlMeta *commonv1.UrlMeta) error
}

func (a *plainAuthInfoInjector) Inject(url string, urlMeta *commonv1.UrlMeta) error {
	return a.inject(url, urlMeta)
}

func NewPlainAuthInfoInjector(
	inject func(url string, urlMeta *commonv1.UrlMeta) error) AuthInfoInjector {
	return &plainAuthInfoInjector{inject: inject}
}
