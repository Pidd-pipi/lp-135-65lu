package repository

import (
	"errors"
	"fmt"
	"time"

	"github.com/givetrack/givetrack/internal/model"
	"gorm.io/gorm"
)

// ErrConflict 状态冲突（如资源已被他人先处理）。
var ErrConflict = errors.New("conflict: resource already processed")

// ProjectRepository 项目数据访问。
type ProjectRepository struct {
	db *gorm.DB
}

func NewProjectRepository(db *gorm.DB) *ProjectRepository {
	return &ProjectRepository{db: db}
}

func (r *ProjectRepository) Create(p *model.Project) error {
	if err := r.db.Create(p).Error; err != nil {
		return fmt.Errorf("create project: %w", err)
	}
	return nil
}

func (r *ProjectRepository) FindByID(id uint) (*model.Project, error) {
	var p model.Project
	err := r.db.Preload("Organization").Preload("Organization.User").First(&p, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find project by id: %w", err)
	}
	return &p, nil
}

func (r *ProjectRepository) Update(p *model.Project) error {
	if err := r.db.Save(p).Error; err != nil {
		return fmt.Errorf("update project: %w", err)
	}
	return nil
}

// List 分页查询项目，支持分类/状态筛选。
func (r *ProjectRepository) List(category, status string, page, pageSize int) ([]model.Project, int64, error) {
	var list []model.Project
	var total int64
	q := r.db.Model(&model.Project{}).Preload("Organization")
	if category != "" && category != "all" {
		q = q.Where("category = ?", category)
	}
	if status != "" {
		q = q.Where("status = ?", status)
	}
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count projects: %w", err)
	}
	if err := q.Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&list).Error; err != nil {
		return nil, 0, fmt.Errorf("list projects: %w", err)
	}
	return list, total, nil
}

// ListByOrg 组织自己的项目。
func (r *ProjectRepository) ListByOrg(orgID uint) ([]model.Project, error) {
	var list []model.Project
	if err := r.db.Where("organization_id = ?", orgID).
		Order("created_at DESC").Find(&list).Error; err != nil {
		return nil, fmt.Errorf("list projects by org: %w", err)
	}
	return list, nil
}

// FindPending 待审核项目列表。
func (r *ProjectRepository) FindPending() ([]model.Project, error) {
	var list []model.Project
	if err := r.db.Preload("Organization").Where("status = ?", "pending").
		Order("created_at DESC").Find(&list).Error; err != nil {
		return nil, fmt.Errorf("find pending projects: %w", err)
	}
	return list, nil
}

// ProjectUpdateRepository 项目进展数据访问。
type ProjectUpdateRepository struct {
	db *gorm.DB
}

func NewProjectUpdateRepository(db *gorm.DB) *ProjectUpdateRepository {
	return &ProjectUpdateRepository{db: db}
}

func (r *ProjectUpdateRepository) Create(u *model.ProjectUpdate) error {
	if err := r.db.Create(u).Error; err != nil {
		return fmt.Errorf("create project update: %w", err)
	}
	return nil
}

// ListApprovedByProject 项目公开动态（仅审核通过）。
func (r *ProjectUpdateRepository) ListApprovedByProject(projectID uint) ([]model.ProjectUpdate, error) {
	var list []model.ProjectUpdate
	if err := r.db.Where("project_id = ? AND status = ?", projectID, "approved").
		Order("created_at DESC").Find(&list).Error; err != nil {
		return nil, fmt.Errorf("list approved project updates: %w", err)
	}
	return list, nil
}

// ListAllByProject 项目全部动态（组织自查，含待审核/已驳回）。
func (r *ProjectUpdateRepository) ListAllByProject(projectID uint) ([]model.ProjectUpdate, error) {
	var list []model.ProjectUpdate
	if err := r.db.Where("project_id = ?", projectID).
		Order("created_at DESC").Find(&list).Error; err != nil {
		return nil, fmt.Errorf("list all project updates: %w", err)
	}
	return list, nil
}

// FindUpdateByID 按 ID 查询单条动态。
func (r *ProjectUpdateRepository) FindUpdateByID(id uint) (*model.ProjectUpdate, error) {
	var u model.ProjectUpdate
	err := r.db.First(&u, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find project update by id: %w", err)
	}
	return &u, nil
}

// FindPendingUpdates 待审核动态（带项目与发起组织信息）。
func (r *ProjectUpdateRepository) FindPendingUpdates() ([]model.ProjectUpdate, error) {
	var list []model.ProjectUpdate
	if err := r.db.Preload("Project").Preload("Project.Organization").
		Where("status = ?", "pending").
		Order("created_at DESC").Find(&list).Error; err != nil {
		return nil, fmt.Errorf("find pending project updates: %w", err)
	}
	return list, nil
}

// ReviewIfPending 仅当动态仍为待审核时原子地写入审核结果。
// 返回 reviewed=true 表示本次调用完成了状态变更；false 表示已被他人先处理。
func (r *ProjectUpdateRepository) ReviewIfPending(id, reviewerID uint, status, comment string, reviewedAt time.Time) (bool, error) {
	res := r.db.Model(&model.ProjectUpdate{}).
		Where("id = ? AND status = ?", id, "pending").
		Updates(map[string]interface{}{
			"status":         status,
			"review_comment": comment,
			"reviewer_id":    reviewerID,
			"reviewed_at":    reviewedAt,
		})
	if res.Error != nil {
		return false, fmt.Errorf("review project update: %w", res.Error)
	}
	return res.RowsAffected == 1, nil
}
