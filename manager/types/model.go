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

package types

type ModelParams struct {
	ID uint `uri:"id" binding:"required"`
}

type CreateModelRequest struct {
	Type        string           `json:"type" binding:"required"`
	BIO         string           `json:"BIO" binding:"omitempty"`
	Version     string           `json:"version"  binding:"required"`
	Evaluation  *ModelEvaluation `json:"evaluation" binding:"omitempty"`
	SchedulerID uint             `json:"scheduler_id" binding:"required"`
}

type UpdateModelRequest struct {
	Type        string           `json:"type" binding:"required"`
	BIO         string           `json:"BIO" binding:"omitempty"`
	State       string           `json:"state" binding:"required,oneof=active inactive"`
	Evaluation  *ModelEvaluation `json:"evaluation" binding:"omitempty"`
	SchedulerID uint             `json:"scheduler_id" binding:"omitempty"`
}

type GetModelsQuery struct {
	Type        string `json:"type" binding:"omitempty"`
	Version     string `json:"version"  binding:"omitempty"`
	State       string `json:"state" binding:"omitempty,oneof=active inactive"`
	SchedulerID uint   `json:"scheduler_id" binding:"omitempty"`
	Page        int    `form:"page" binding:"omitempty,gte=1"`
	PerPage     int    `form:"per_page" binding:"omitempty,gte=1,lte=1000"`
}

type ModelEvaluation struct {
	GNNModelEvaluation *GNNModelEvaluation `json:"gnn_model_evaluation" binding:"omitempty"`
	MLPModelEvaluation *MLPModelEvaluation `json:"mlp_model_evaluation" binding:"omitempty"`
}

type GNNModelEvaluation struct {
	Recall    float64 `json:"recall" binding:"omitempty"`
	Precision float64 `json:"precision" binding:"omitempty"`
	F1Score   float64 `json:"f1_score" binding:"omitempty"`
}

type MLPModelEvaluation struct {
	Mse float64 `json:"mse" binding:"omitempty"`
	Mae float64 `json:"mae" binding:"omitempty"`
}
