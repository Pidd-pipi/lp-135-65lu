package repository

import (
	"errors"
	"fmt"
	"time"

	"github.com/givetrack/givetrack/internal/model"
	"gorm.io/gorm"
)

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

func (r *ProjectUpdateRepository) FindByID(id uint) (*model.ProjectUpdate, error) {
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

// ListByProject 项目动态列表。
// statuses 为空时查询全部状态；传入状态时只返回对应动态（如公开页只看 approved）。
func (r *ProjectUpdateRepository) ListByProject(projectID uint, statuses ...string) ([]model.ProjectUpdate, error) {
	var list []model.ProjectUpdate
	q := r.db.Where("project_id = ?", projectID)
	if len(statuses) > 0 {
		q = q.Where("status IN ?", statuses)
	}
	if err := q.Order("created_at DESC").Find(&list).Error; err != nil {
		return nil, fmt.Errorf("list project updates: %w", err)
	}
	return list, nil
}

// ListByOrg 组织自己项目下的全部动态（含待审核/已驳回）。
func (r *ProjectUpdateRepository) ListByOrg(orgID uint) ([]model.ProjectUpdate, error) {
	var list []model.ProjectUpdate
	if err := r.db.
		Preload("Project").
		Joins("JOIN projects ON projects.id = project_updates.project_id").
		Where("projects.organization_id = ?", orgID).
		Order("project_updates.created_at DESC").
		Find(&list).Error; err != nil {
		return nil, fmt.Errorf("list project updates by org: %w", err)
	}
	return list, nil
}

// FindPending 待审核动态。
func (r *ProjectUpdateRepository) FindPending() ([]model.ProjectUpdate, error) {
	var list []model.ProjectUpdate
	if err := r.db.
		Preload("Project").Preload("Project.Organization").
		Where("project_updates.status = ?", "pending").
		Order("project_updates.created_at DESC").
		Find(&list).Error; err != nil {
		return nil, fmt.Errorf("find pending project updates: %w", err)
	}
	return list, nil
}

// Review 原子审核一条动态。
//
// 仅当数据库中当前状态仍为 pending 时条件更新才会命中一行，
// 因此两个管理员并发处理同一条动态时，只有先到的请求 RowsAffected=1，
// 后到的请求 RowsAffected=0 返回 ErrConflict，无法重复改状态，
// 也不会产生第二条审核结论（审核信息在同一条 UPDATE 中写入）。
func (r *ProjectUpdateRepository) Review(id, reviewerID uint, status, comment string) (*model.ProjectUpdate, error) {
	now := time.Now()
	res := r.db.Model(&model.ProjectUpdate{}).
		Where("id = ? AND status = ?", id, "pending").
		Updates(map[string]interface{}{
			"status":         status,
			"review_comment": comment,
			"reviewer_id":    reviewerID,
			"reviewed_at":    now,
		})
	if res.Error != nil {
		return nil, fmt.Errorf("review project update: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		// 不存在返回 NotFound；存在但已被处理返回 Conflict，由调用方区分提示。
		if _, err := r.FindByID(id); errors.Is(err, ErrNotFound) {
			return nil, ErrNotFound
		} else if err != nil {
			return nil, err
		}
		return nil, ErrConflict
	}
	return r.FindByID(id)
}
