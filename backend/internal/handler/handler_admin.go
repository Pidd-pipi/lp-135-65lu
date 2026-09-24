package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/givetrack/givetrack/internal/constants"
	"github.com/givetrack/givetrack/internal/repository"
	"github.com/givetrack/givetrack/internal/service"
	"github.com/givetrack/givetrack/internal/util"
)

// AdminHandler 平台审核处理器。
type AdminHandler struct {
	adminSvc *service.AdminService
}

func NewAdminHandler(adminSvc *service.AdminService) *AdminHandler {
	return &AdminHandler{adminSvc: adminSvc}
}

// PendingProjects 待审核项目。
func (h *AdminHandler) PendingProjects(c *gin.Context) {
	list, err := h.adminSvc.PendingProjects()
	if err != nil {
		util.FailError(c, err)
		return
	}
	util.OK(c, gin.H{"projects": list})
}

// ReviewProject 审核项目。
func (h *AdminHandler) ReviewProject(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		util.Fail(c, http.StatusBadRequest, 40000, "invalid project id")
		return
	}
	var req struct {
		Status  string `json:"status" binding:"required"`
		Comment string `json:"comment"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		util.Fail(c, http.StatusBadRequest, 42200, err.Error())
		return
	}
	p, err := h.adminSvc.ReviewProject(c.GetUint("user_id"), uint(id), req.Status, req.Comment)
	if err != nil {
		util.FailError(c, err)
		return
	}
	util.OK(c, gin.H{"project": p})
}

// PendingOrganizations 待审核组织。
func (h *AdminHandler) PendingOrganizations(c *gin.Context) {
	list, err := h.adminSvc.PendingOrganizations()
	if err != nil {
		util.FailError(c, err)
		return
	}
	util.OK(c, gin.H{"organizations": list})
}

// PendingUpdates 待审核项目动态。
func (h *AdminHandler) PendingUpdates(c *gin.Context) {
	list, err := h.adminSvc.PendingUpdates()
	if err != nil {
		util.FailError(c, err)
		return
	}
	util.OK(c, gin.H{"updates": list})
}

// ReviewUpdate 审核项目动态（通过/驳回，驳回须填写原因；并发下先到者生效）。
func (h *AdminHandler) ReviewUpdate(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		util.Fail(c, http.StatusBadRequest, 40000, "invalid update id")
		return
	}
	var req struct {
		Status  string `json:"status" binding:"required"`
		Comment string `json:"comment"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		util.Fail(c, http.StatusBadRequest, 42200, err.Error())
		return
	}
	u, err := h.adminSvc.ReviewUpdate(c.GetUint("user_id"), uint(id), req.Status, req.Comment)
	if err != nil {
		if errors.Is(err, repository.ErrConflict) {
			// 已被他人先处理：返回 409 与当前处理结果，后到者只能看到结果、不能再改
			c.JSON(http.StatusConflict, gin.H{
				"code":    constants.CodeConflict,
				"message": "该动态已被其他管理员处理",
				"data":    gin.H{"update": u},
			})
			return
		}
		util.FailError(c, err)
		return
	}
	util.OK(c, gin.H{"update": u})
}

// ReviewOrganization 审核组织。
func (h *AdminHandler) ReviewOrganization(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		util.Fail(c, http.StatusBadRequest, 40000, "invalid organization id")
		return
	}
	var req struct {
		Status  string `json:"status" binding:"required"`
		Comment string `json:"comment"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		util.Fail(c, http.StatusBadRequest, 42200, err.Error())
		return
	}
	o, err := h.adminSvc.ReviewOrganization(c.GetUint("user_id"), uint(id), req.Status, req.Comment)
	if err != nil {
		util.FailError(c, err)
		return
	}
	util.OK(c, gin.H{"organization": o})
}
