package repository

import (
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/givetrack/givetrack/internal/constants"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func newMockGorm(t *testing.T) (sqlmock.Sqlmock, *gorm.DB) {
	t.Helper()
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	if err != nil {
		t.Fatalf("open sqlmock: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	// 跳过 GORM 默认事务包装：待验证的原子性来自单条条件 UPDATE，与外层事务无关。
	gdb, err := gorm.Open(mysql.New(mysql.Config{Conn: db, SkipInitializeWithVersion: true}), &gorm.Config{SkipDefaultTransaction: true})
	if err != nil {
		t.Fatalf("open gorm: %v", err)
	}
	return mock, gdb
}

// 审核更新列（按 GORM 对 map 的排序：review_comment, reviewed_at, reviewer_id, status）+ WHERE 两参数。
func expectReviewExec(mock sqlmock.Sqlmock, affected int64) {
	mock.ExpectExec(`(?is)UPDATE .project_updates. SET .*WHERE id = . AND status = .`).
		WithArgs(
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(),
		).
		WillReturnResult(sqlmock.NewResult(0, affected))
}

func updateColumns(status, comment string) *sqlmock.Rows {
	return sqlmock.NewRows([]string{
		"id", "project_id", "title", "content", "images",
		"status", "review_comment", "reviewer_id", "reviewed_at", "created_at",
	}).AddRow(
		1, 10, "进展", "内容", "",
		status, comment, 7, sql.NullTime{Valid: false}, time.Now(),
	)
}

func TestProjectUpdateReview_Success(t *testing.T) {
	mock, gdb := newMockGorm(t)
	repo := NewProjectUpdateRepository(gdb)
	expectReviewExec(mock, 1)
	mock.ExpectQuery(`(?is)SELECT .* FROM .project_updates.`).
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnRows(updateColumns(constants.UpdateApproved, ""))

	u, err := repo.Review(1, 7, constants.UpdateApproved, "")
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if u.Status != constants.UpdateApproved || u.ReviewerID != 7 {
		t.Fatalf("unexpected update: %+v", u)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// 后到的管理员：条件 UPDATE 命中 0 行，动态已存在（已被先到者处理），必须返回 ErrConflict。
func TestProjectUpdateReview_ConcurrentConflict(t *testing.T) {
	mock, gdb := newMockGorm(t)
	repo := NewProjectUpdateRepository(gdb)
	expectReviewExec(mock, 0)
	mock.ExpectQuery(`(?is)SELECT .* FROM .project_updates.`).
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnRows(updateColumns(constants.UpdateApproved, ""))

	_, err := repo.Review(1, 8, constants.UpdateRejected, "想重复审核")
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("expected ErrConflict, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// 动态不存在：命中 0 行且查不到，返回 ErrNotFound。
func TestProjectUpdateReview_NotFound(t *testing.T) {
	mock, gdb := newMockGorm(t)
	repo := NewProjectUpdateRepository(gdb)
	expectReviewExec(mock, 0)
	mock.ExpectQuery(`(?is)SELECT .* FROM .project_updates.`).
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnError(gorm.ErrRecordNotFound)

	_, err := repo.Review(99, 7, constants.UpdateApproved, "")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
