package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

const swaggerJSON = `{
  "swagger": "2.0",
  "info": {
    "title": "农机调度管理系统 API",
    "description": "农机资源管理、作业任务调度、实时轨迹监控、作业统计与维修保养提醒。",
    "version": "1.0.0"
  },
  "basePath": "/api/v1",
  "schemes": ["http", "ws"],
  "paths": {
    "/auth/login": { "post": { "summary": "登录", "tags": ["auth"] } },
    "/auth/me": { "get": { "summary": "当前用户", "tags": ["auth"] } },
    "/dashboard/overview": { "get": { "summary": "调度看板总览", "tags": ["dashboard"] } },
    "/dashboard/tasks/{id}/dispatch": { "post": { "summary": "一键派单（在途农机返回 409 拒绝）", "tags": ["dashboard"] } },
    "/dashboard/reports/work/export": { "get": { "summary": "作业报表导出", "tags": ["dashboard"] } },
    "/transfers": {
      "post": { "summary": "发起转场（仅空闲农机）", "tags": ["transfer"] },
      "get": { "summary": "转场记录，可按 machineCode 筛选", "tags": ["transfer"] }
    },
    "/transfers/{id}/arrive": { "post": { "summary": "到达确认（更新所属地块并恢复可派）", "tags": ["transfer"] } },
    "/transfers/{id}/cancel": { "post": { "summary": "申请人取消（回到原地块）", "tags": ["transfer"] } }
  }
}`

// SwaggerJSON 提供 swagger.json。
func (h *HealthHandler) SwaggerJSON(c *gin.Context) {
	c.Header("Content-Type", "application/json")
	c.String(http.StatusOK, swaggerJSON)
}
