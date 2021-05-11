package handler

import (
	"context"
	"net/http"

	"d7y.io/dragonfly/v2/manager/apis/v2/types"
	"d7y.io/dragonfly/v2/manager/store"
	"d7y.io/dragonfly/v2/pkg/dfcodes"
	"d7y.io/dragonfly/v2/pkg/dferrors"
	"github.com/gin-gonic/gin"
	"gopkg.in/errgo.v2/fmt/errors"
)

// CreateSchedulerInstance godoc
// @Summary Add scheduler instance
// @Description add by json config
// @Tags SchedulerInstance
// @Accept  json
// @Produce  json
// @Param instance body types.SchedulerInstance true "Scheduler instance"
// @Success 200 {object} types.SchedulerInstance
// @Failure 400 {object} HTTPError
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /scheduler/instances [post]
func (handler *Handler) CreateSchedulerInstance(ctx *gin.Context) {
	var instance types.SchedulerInstance
	if err := ctx.ShouldBindJSON(&instance); err != nil {
		NewError(ctx, http.StatusBadRequest, err)
		return
	}

	if err := checkSchedulerInstanceValidate(&instance); err != nil {
		NewError(ctx, http.StatusBadRequest, err)
		return
	}

	retInstance, err := handler.server.AddSchedulerInstance(context.TODO(), &instance)
	if err == nil {
		ctx.JSON(http.StatusOK, retInstance)
	} else if dferrors.CheckError(err, dfcodes.InvalidResourceType) {
		NewError(ctx, http.StatusBadRequest, err)
	} else if dferrors.CheckError(err, dfcodes.ManagerStoreError) {
		NewError(ctx, http.StatusInternalServerError, err)
	} else {
		NewError(ctx, http.StatusNotFound, err)
	}
}

// DestroySchedulerInstance godoc
// @Summary Delete scheduler instance
// @Description Delete by instanceId
// @Tags SchedulerInstance
// @Accept  json
// @Produce  json
// @Param  id path string true "InstanceID"
// @Success 200 {string} string
// @Failure 400 {object} HTTPError
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /scheduler/instances/{id} [delete]
func (handler *Handler) DestroySchedulerInstance(ctx *gin.Context) {
	var uri types.SchedulerInstanceURI
	if err := ctx.ShouldBindUri(&uri); err != nil {
		NewError(ctx, http.StatusBadRequest, err)
		return
	}

	retInstance, err := handler.server.DeleteSchedulerInstance(context.TODO(), uri.InstanceID)
	if err == nil {
		if retInstance != nil {
			ctx.JSON(http.StatusOK, "success")
		} else {
			NewError(ctx, http.StatusNotFound, errors.Newf("scheduler instance not found, id %s", uri.InstanceID))
		}
	} else if dferrors.CheckError(err, dfcodes.InvalidResourceType) {
		NewError(ctx, http.StatusBadRequest, err)
	} else if dferrors.CheckError(err, dfcodes.ManagerStoreError) {
		NewError(ctx, http.StatusInternalServerError, err)
	} else {
		NewError(ctx, http.StatusNotFound, err)
	}
}

// UpdateSchedulerInstance godoc
// @Summary Update scheduler instance
// @Description Update by json scheduler instance
// @Tags SchedulerInstance
// @Accept  json
// @Produce  json
// @Param  id path string true "InstanceID"
// @Param  Instance body types.SchedulerInstance true "SchedulerInstance"
// @Success 200 {string} string
// @Failure 400 {object} HTTPError
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /scheduler/instances/{id} [post]
func (handler *Handler) UpdateSchedulerInstance(ctx *gin.Context) {
	var uri types.SchedulerInstanceURI
	if err := ctx.ShouldBindUri(&uri); err != nil {
		NewError(ctx, http.StatusBadRequest, err)
		return
	}

	var instance types.SchedulerInstance
	if err := ctx.ShouldBindJSON(&instance); err != nil {
		NewError(ctx, http.StatusBadRequest, err)
		return
	}

	if err := checkSchedulerInstanceValidate(&instance); err != nil {
		NewError(ctx, http.StatusBadRequest, err)
		return
	}

	instance.InstanceID = uri.InstanceID
	_, err := handler.server.UpdateSchedulerInstance(context.TODO(), &instance)
	if err == nil {
		ctx.JSON(http.StatusOK, "success")
	} else if dferrors.CheckError(err, dfcodes.InvalidResourceType) {
		NewError(ctx, http.StatusBadRequest, err)
	} else if dferrors.CheckError(err, dfcodes.ManagerStoreNotFound) {
		NewError(ctx, http.StatusNotFound, err)
	} else if dferrors.CheckError(err, dfcodes.ManagerStoreError) {
		NewError(ctx, http.StatusInternalServerError, err)
	} else {
		NewError(ctx, http.StatusNotFound, err)
	}
}

// GetSchedulerInstance godoc
// @Summary Get scheduler instance
// @Description Get scheduler instance by InstanceID
// @Tags SchedulerInstance
// @Accept  json
// @Produce  json
// @Param id path string true "InstanceID"
// @Success 200 {object} types.SchedulerInstance
// @Failure 400 {object} HTTPError
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /scheduler/instances/{id} [get]
func (handler *Handler) GetSchedulerInstance(ctx *gin.Context) {
	var uri types.SchedulerInstanceURI
	if err := ctx.ShouldBindUri(&uri); err != nil {
		NewError(ctx, http.StatusBadRequest, err)
		return
	}

	retInstance, err := handler.server.GetSchedulerInstance(context.TODO(), uri.InstanceID)
	if err == nil {
		ctx.JSON(http.StatusOK, &retInstance)
	} else if dferrors.CheckError(err, dfcodes.InvalidResourceType) {
		NewError(ctx, http.StatusBadRequest, err)
	} else if dferrors.CheckError(err, dfcodes.ManagerStoreNotFound) {
		NewError(ctx, http.StatusNotFound, err)
	} else if dferrors.CheckError(err, dfcodes.ManagerStoreError) {
		NewError(ctx, http.StatusInternalServerError, err)
	} else {
		NewError(ctx, http.StatusNotFound, err)
	}
}

// ListSchedulerInstances godoc
// @Summary List scheduler instances
// @Description List by object
// @Tags SchedulerInstance
// @Accept  json
// @Produce  json
// @Param marker query int true "begin marker of current page" default(0)
// @Param maxItemCount query int true "return max item count, default 10, max 50" default(10) minimum(10) maximum(50)
// @Success 200 {object} types.ListSchedulerInstancesResponse
// @Failure 400 {object} HTTPError
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /scheduler/instances [get]
func (handler *Handler) ListSchedulerInstances(ctx *gin.Context) {
	var query types.ListQuery
	if err := ctx.ShouldBindQuery(&query); err != nil {
		NewError(ctx, http.StatusBadRequest, err)
		return
	}

	instances, err := handler.server.ListSchedulerInstances(context.TODO(), store.WithMarker(query.Marker, query.MaxItemCount))
	if err == nil {
		if len(instances) > 0 {
			ctx.JSON(http.StatusOK, &types.ListSchedulerInstancesResponse{Instances: instances})
		} else {
			NewError(ctx, http.StatusNotFound, errors.Newf("list scheduler instances empty, marker %d, maxItemCount %d", query.Marker, query.MaxItemCount))
		}
	} else if dferrors.CheckError(err, dfcodes.InvalidResourceType) {
		NewError(ctx, http.StatusBadRequest, err)
	} else if dferrors.CheckError(err, dfcodes.ManagerStoreError) {
		NewError(ctx, http.StatusInternalServerError, err)
	} else {
		NewError(ctx, http.StatusNotFound, err)
	}
}

func checkSchedulerInstanceValidate(instance *types.SchedulerInstance) (err error) {
	return nil
}
