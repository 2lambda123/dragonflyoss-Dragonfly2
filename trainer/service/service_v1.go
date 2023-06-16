/*
 *     Copyright 2023 The Dragonfly Authors
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

package service

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"hash"
	"io"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	trainerv1 "d7y.io/api/pkg/apis/trainer/v1"

	logger "d7y.io/dragonfly/v2/internal/dflog"
	"d7y.io/dragonfly/v2/trainer/config"
	"d7y.io/dragonfly/v2/trainer/storage"
)

const (
	// AlgorithmSHA1 is sha1 algorithm name of hash.
	AlgorithmSHA1 = "sha1"

	// AlgorithmSHA256 is sha256 algorithm name of hash.
	AlgorithmSHA256 = "sha256"

	// AlgorithmSHA512 is sha512 algorithm name of hash.
	AlgorithmSHA512 = "sha512"

	// AlgorithmMD5 is md5 algorithm name of hash.
	AlgorithmMD5 = "md5"

	// DefaultHashAlgorithm is the default hash algorithm used to generate the digest of the model key.
	DefaultHashAlgorithm = "sha256"
)

// V1 is the interface for v1 version of the service.
type V1 struct {
	// Trainer service config.
	config *config.Config

	// Storage Interface.
	storage storage.Storage
}

// New v1 version of service instance.
func NewV1(
	cfg *config.Config,
	storage storage.Storage,

) *V1 {
	return &V1{
		config:  cfg,
		storage: storage,
	}
}

func (v *V1) Train(stream trainerv1.Trainer_TrainServer) error {
	var (
		modelKey    string
		initialized bool
	)

	for {
		req, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				logger.Infof("receive streaming requests successfully")
				// TODO (fyx) Add training logiic.
				if err := stream.SendAndClose(new(emptypb.Empty)); err != nil {
					logger.Infof("Send and close error %s", err.Error())
					return err
				}

				initialized = false
				return nil
			}

			if !initialized {
				logger.Errorf("receive error at the time of beginning: %s", err.Error())
				return err
			}

			// If receive stream request fails, delete the file of downloads and
			// network topologies according to given model key.
			logger.Errorf("receive error: %s", err.Error())
			if err := v.storage.ClearDownload(modelKey); err != nil {
				logger.Errorf("clear downloads error: %s", err.Error())
				return err
			}

			if err := v.storage.ClearNetworkTopology(modelKey); err != nil {
				logger.Errorf("clear network topologies error: %s", err.Error())
				return err
			}

			return err
		}

		if !initialized {
			initialized = true

			//Initialize modelKey.
			modelKey, err = v.createModelKey(req.Hostname, req.Ip, uint(req.ClusterId), DefaultHashAlgorithm)
			if err != nil {
				logger.Errorf("create model key error: %s", err.Error())
				return err
			}
		}

		switch trainRequest := req.GetRequest().(type) {
		case *trainerv1.TrainRequest_TrainGnnRequest:
			logger.Infof("receive TrainRequest_TrainGnnRequest: %#v", trainRequest.TrainGnnRequest)
			if err := v.handleTrainGNNRequest(modelKey, trainRequest.TrainGnnRequest.Dataset); err != nil {
				logger.Errorf("handle network topologies error: %s", err.Error())
				if err := v.storage.ClearNetworkTopology(modelKey); err != nil {
					logger.Errorf("clear network topologies error: %s", err.Error())
					return err
				}

				return err
			}
		case *trainerv1.TrainRequest_TrainMlpRequest:
			logger.Infof("receive TrainRequest_TrainMlpRequest: %#v", trainRequest.TrainMlpRequest)
			if err := v.handleTrainMLPRequest(modelKey, trainRequest.TrainMlpRequest.Dataset); err != nil {
				logger.Errorf("handle downloads error: %s", err.Error())
				if err := v.storage.ClearDownload(modelKey); err != nil {
					logger.Errorf("clear downloads error: %s", err.Error())
					return err
				}

				return err
			}
		default:
			msg := fmt.Sprintf("receive unknown request: %#v", trainRequest)
			logger.Error(msg)
			return status.Error(codes.FailedPrecondition, msg)
		}
	}
}

func (v *V1) handleTrainMLPRequest(modelKey string, dataset []byte) error {
	file, err := v.storage.OpenDownload(modelKey)
	if err != nil {
		return err
	}
	defer file.Close()

	if _, err = file.Write(dataset); err != nil {
		return err
	}

	return nil
}

func (v *V1) handleTrainGNNRequest(modelKey string, dataset []byte) error {
	file, err := v.storage.OpenNetworkTopology(modelKey)
	if err != nil {
		return err
	}
	defer func() {
		if err := file.Close(); err != nil {
			logger.Error(err)
		}
	}()

	if _, err = file.Write(dataset); err != nil {
		return err
	}

	return nil
}

func (v *V1) createModelKey(hostname, ip string, clusterID uint, hashAlgorithm string) (string, error) {
	var h hash.Hash
	switch hashAlgorithm {
	case AlgorithmSHA1:
		h = sha1.New()
	case AlgorithmSHA256:
		h = sha256.New()
	case AlgorithmSHA512:
		h = sha512.New()
	case AlgorithmMD5:
		h = md5.New()
	default:
		return "", fmt.Errorf("unsupport hash method: %s", hashAlgorithm)
	}

	if _, err := h.Write([]byte(fmt.Sprintf("%s-%s-%d", hostname, ip, clusterID))); err != nil {
		return "", err
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}
