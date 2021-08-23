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

package handlers

import (
	"net/http"

	"d7y.io/dragonfly/v2/manager/types"
	"github.com/gin-gonic/gin"
)

// @Summary Get Permissions
// @Description Get Permissions
// @Tags permission
// @Produce json
// @Success 200 {object} RoutesInfo
// @Failure 400 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /permissions [get]

func (h *Handlers) GetPermissions(g *gin.Engine) func(ctx *gin.Context) {
	return func(ctx *gin.Context) {

		permissionGroups := h.Service.GetPermissions(g)

		ctx.JSON(http.StatusOK, permissionGroups)
	}
}

// @Summary Create Role
// @Description Create Role by json config
// @Tags role
// @Accept json
// @Produce json
// @Success 200
// @Failure 400 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /roles [post]

func (h *Handlers) CreateRole(ctx *gin.Context) {
	var json types.CreateRolePermissionRequest
	if err := ctx.ShouldBindJSON(&json); err != nil {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"errors": err.Error()})
		return
	}
	err := h.Service.CreateRole(json)
	if err != nil {
		ctx.Error(err)
		return
	}
	ctx.Status(http.StatusOK)
}

// @Summary Update Role
// @Description Remove Role Permission by json config
// @Tags role
// @Accept json
// @Produce json
// @Success 200
// @Failure 400 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /roles/:role_name/permission [delete]

func (h *Handlers) RemoveRolePermission(ctx *gin.Context) {

	var params types.RoleParams
	if err := ctx.ShouldBindUri(&params); err != nil {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"errors": err.Error()})
		return
	}
	var json types.ObjectPermission
	if err := ctx.ShouldBindJSON(&json); err != nil {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"errors": err.Error()})
		return
	}
	err := h.Service.RemoveRolePermission(params.RoleName, json)
	if err != nil {
		ctx.Error(err)
		return
	}
	ctx.Status(http.StatusOK)
}

// @Summary Update Role
// @Description Add Role Permission by json config
// @Tags role
// @Accept json
// @Produce json
// @Success 200
// @Failure 400 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /roles/:role_name/permission [post]

func (h *Handlers) AddRolePermission(ctx *gin.Context) {

	var params types.RoleParams
	if err := ctx.ShouldBindUri(&params); err != nil {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"errors": err.Error()})
		return
	}
	var json types.ObjectPermission
	if err := ctx.ShouldBindJSON(&json); err != nil {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"errors": err.Error()})
		return
	}
	err := h.Service.AddRolePermission(params.RoleName, json)
	if err != nil {
		ctx.Error(err)
		return
	}
	ctx.Status(http.StatusOK)
}

// @Summary Get Roles
// @Description Get Roles by name
// @Tags role
// @Accept text
// @Produce json
// @Success 200
// @Failure 400 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /roles [get]

func (h *Handlers) GetRoles(ctx *gin.Context) {
	roles := h.Service.GetRoles()
	ctx.JSON(http.StatusOK, roles)
}

// @Summary Get Role
// @Description Get Role
// @Tags permission
// @Accept text
// @Produce json
// @Success 200
// @Failure 400 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /roles/:role_name [get]

func (h *Handlers) GetRole(ctx *gin.Context) {
	var params types.RoleParams
	if err := ctx.ShouldBindUri(&params); err != nil {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"errors": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, h.Service.GetRole(params.RoleName))
}

// @Summary Destroy Permission
// @Description Destroy Permission by json config
// @Tags permission
// @Accept json
// @Produce json
// @Success 200
// @Failure 400 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /role/:role_name [delete]

func (h *Handlers) DestroyRole(ctx *gin.Context) {
	var params types.RoleParams
	if err := ctx.ShouldBindUri(&params); err != nil {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"errors": err.Error()})
		return
	}
	err := h.Service.DestroyRole(params.RoleName)
	if err != nil {
		ctx.Error(err)
		return
	}
	ctx.Status(http.StatusOK)
}
