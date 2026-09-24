package service

import (
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/givetrack/givetrack/internal/constants"
	"github.com/givetrack/givetrack/internal/model"
	"github.com/givetrack/givetrack/internal/repository"
)

// AdminService 平台审核服务。
type AdminService struct {
	projectRepo *repository.ProjectRepository
	updateRepo  *repository.ProjectUpdateRepository
	orgRepo     *repository.OrganizationRepository
	reviewRepo  *repository.AdminReviewRepository
	logger      *slog.Logger
}

func NewAdminService(projectRepo *repository.ProjectRepository, updateRepo *repository.ProjectUpdateRepository, orgRepo *repository.OrganizationRepository, reviewRepo *repository.AdminReviewRepository, logger *slog.Logger) *AdminService {
	return &AdminService{projectRepo: projectRepo, updateRepo: updateRepo, orgRepo: orgRepo, reviewRepo: reviewRepo, logger: logger}
}

// PendingProjects 待审核项目列表。
func (s *AdminService) PendingProjects() ([]model.Project, error) {
	return s.projectRepo.FindPending()
}

// ReviewProject 审核项目。
func (s *AdminService) ReviewProject(adminID, projectID uint, status, comment string) (*model.Project, error) {
	if status != constants.ProjectApproved && status != constants.ProjectRejected {
		return nil, fmt.Errorf("invalid review status")
	}
	p, err := s.projectRepo.FindByID(projectID)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, err
	}
	if err != nil {
		return nil, err
	}
	p.Status = status
	if err := s.projectRepo.Update(p); err != nil {
		return nil, err
	}
	if err := s.reviewRepo.Create(&model.AdminReview{
		ProjectID:  projectID,
		ReviewerID: adminID,
		Status:     status,
		Comment:    comment,
	}); err != nil {
		return nil, err
	}
	s.logger.Info("project reviewed", "projectId", projectID, "status", status)
	return p, nil
}

// PendingOrganizations 待审核组织列表。
func (s *AdminService) PendingOrganizations() ([]model.Organization, error) {
	return s.orgRepo.FindPending()
}

// PendingUpdates 待审核项目动态列表。
func (s *AdminService) PendingUpdates() ([]model.ProjectUpdate, error) {
	return s.updateRepo.FindPendingUpdates()
}

// validateUpdateReviewInput 校验动态审核入参：状态合法，驳回必须填写原因。
func validateUpdateReviewInput(status, comment string) error {
	if status != constants.UpdateApproved && status != constants.UpdateRejected {
		return fmt.Errorf("invalid review status")
	}
	if status == constants.UpdateRejected && strings.TrimSpace(comment) == "" {
		return fmt.Errorf("reject reason is required")
	}
	return nil
}

// ReviewUpdate 审核项目动态。
// 依赖数据库条件更新保证并发安全：只有仍处于 pending 的动态才能被审核，
// 两位管理员同时处理时，先到的完成状态变更，后到的收到 ErrConflict，
// 不会重复改状态，也不会产生第二份审核结论。
func (s *AdminService) ReviewUpdate(adminID, updateID uint, status, comment string) (*model.ProjectUpdate, error) {
	if err := validateUpdateReviewInput(status, comment); err != nil {
		return nil, err
	}
	if _, err := s.updateRepo.FindUpdateByID(updateID); err != nil {
		return nil, err
	}
	reviewed, err := s.updateRepo.ReviewIfPending(updateID, adminID, status, strings.TrimSpace(comment), time.Now())
	if err != nil {
		return nil, err
	}
	if !reviewed {
		// 已被其他管理员先处理，返回当前处理结果，调用方不得再改。
		current, err := s.updateRepo.FindUpdateByID(updateID)
		if err != nil {
			return nil, err
		}
		s.logger.Info("project update already reviewed", "updateId", updateID, "status", current.Status, "reviewerId", current.ReviewerID)
		return current, repository.ErrConflict
	}
	u, err := s.updateRepo.FindUpdateByID(updateID)
	if err != nil {
		return nil, err
	}
	s.logger.Info("project update reviewed", "updateId", updateID, "status", status, "reviewerId", adminID)
	return u, nil
}

// ReviewOrganization 审核组织。
func (s *AdminService) ReviewOrganization(adminID, orgID uint, status, comment string) (*model.Organization, error) {
	if status != constants.OrgApproved && status != constants.OrgRejected {
		return nil, fmt.Errorf("invalid review status")
	}
	o, err := s.orgRepo.FindByID(orgID)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, err
	}
	if err != nil {
		return nil, err
	}
	o.Status = status
	if err := s.orgRepo.Update(o); err != nil {
		return nil, err
	}
	if err := s.reviewRepo.Create(&model.AdminReview{
		OrganizationID: orgID,
		ReviewerID:     adminID,
		Status:         status,
		Comment:        comment,
	}); err != nil {
		return nil, err
	}
	s.logger.Info("organization reviewed", "orgId", orgID, "status", status)
	return o, nil
}
