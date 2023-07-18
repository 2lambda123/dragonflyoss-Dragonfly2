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

package training

// MLPObservation contains content for the observed data for download file.
type MLPObservation struct {
	// FinishedPieceScore is feature, 0.0~unlimited larger and better.
	FinishedPieceScore float64 `csv:"finishedPieceScore"`

	// FreeUploadScore is feature, 0.0~unlimited larger and better.
	FreeUploadScore float64 `csv:"freeUploadScore"`

	// UploadSuccessScore is feature, 0.0~unlimited larger and better.
	UploadSuccessScore float64 `csv:"uploadPieceCount"`

	// IDCAffinityScore is feature, 0.0~unlimited larger and better.
	IDCAffinityScore float64 `csv:"idcAffinityScore"`

	// LocationAffinityScore is feature, 0.0~unlimited larger and better.
	LocationAffinityScore float64 `csv:"locationAffinityScore"`

	// MaxBandwidth is label, calculated by length and cost.
	MaxBandwidth float64 `csv:"maxBandwidth"`
}

// GNNVertexObservation contains content for the observed vertex data for network topology file.
type GNNVertexObservation struct {
	// hostID is host id.
	HostID string `csv:"hostID"`

	// IP is feature.
	IP []uint32 `csv:"ip" csv[]:"32"`

	// Location is feature.
	Location []uint32 `csv:"location" csv[]:"32"`

	// IDC is feature.
	IDC []uint32 `csv:"idc" csv[]:"32"`
}

// GNNEdgeObservation contains content for the observed edge data for network topology file.
type GNNEdgeObservation struct {
	// SrcHostID is source host id.
	SrcHostID string `csv:"srcHostID"`

	// DestHostID is destination host id.
	DestHostID string `csv:"destHostID"`

	// AverageRTT is feature that indicates the average round-trip time.
	AverageRTT int64 `csv:"averageRTT"`

	// MaxBandwidth is feature that indicates the maximum bandwidth.
	MaxBandwidth float64 `csv:"maxBandwidth"`
}