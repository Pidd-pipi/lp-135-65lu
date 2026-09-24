package repository

import (
	"sync"
	"testing"
	"time"

	"github.com/givetrack/givetrack/internal/model"
	sqlite "gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	// 文件模式 + 忙等待，便于模拟多连接并发写入
	dsn := "file:" + t.TempDir() + "/test.db?_busy_timeout=5000&_journal_mode=WAL"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.Project{}, &model.ProjectUpdate{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(4)
	return db
}

// TestReviewIfPendingConcurrent 两位管理员并发审核同一动态：
// 只有一人完成状态变更，另一人得到 false，且只留有一份审核结论。
func TestReviewIfPendingConcurrent(t *testing.T) {
	db := newTestDB(t)
	repo := NewProjectUpdateRepository(db)

	p := &model.Project{OrganizationID: 1, Title: "P", Category: "education", TargetAmount: 1000, Status: "approved"}
	if err := db.Create(p).Error; err != nil {
		t.Fatalf("create project: %v", err)
	}
	u := &model.ProjectUpdate{ProjectID: p.ID, Title: "U", Status: "pending"}
	if err := repo.Create(u); err != nil {
		t.Fatalf("create update: %v", err)
	}

	const n = 2
	results := make([]bool, n)
	statuses := make([]string, n)
	comments := make([]string, n)
	reviewers := make([]uint, n)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			status := "rejected"
			comment := "原因"
			reviewer := uint(100 + i)
			if i == 0 {
				status = "approved"
				comment = ""
				reviewer = 1
			}
			ok, err := repo.ReviewIfPending(u.ID, reviewer, status, comment, time.Now())
			if err != nil {
				t.Errorf("worker %d review: %v", i, err)
				return
			}
			results[i] = ok
			statuses[i] = status
			comments[i] = comment
			reviewers[i] = reviewer
		}(i)
	}
	close(start)
	wg.Wait()

	winners, losers := 0, 0
	for _, ok := range results {
		if ok {
			winners++
		} else {
			losers++
		}
	}
	if winners != 1 || losers != 1 {
		t.Fatalf("want exactly 1 winner and 1 loser, got winners=%d losers=%d (%v)", winners, losers, results)
	}

	var final model.ProjectUpdate
	if err := db.First(&final, u.ID).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if final.Status == "pending" {
		t.Fatal("update still pending after concurrent reviews")
	}
	// 最终状态必须等于先到者的结论，而不是后到者的
	winnerIdx := -1
	for i, ok := range results {
		if ok {
			winnerIdx = i
		}
	}
	if final.Status != statuses[winnerIdx] {
		t.Fatalf("final status = %q, want winner status %q", final.Status, statuses[winnerIdx])
	}
	if final.ReviewComment != comments[winnerIdx] {
		t.Fatalf("final comment = %q, want %q", final.ReviewComment, comments[winnerIdx])
	}
	if final.ReviewerID != reviewers[winnerIdx] {
		t.Fatalf("final reviewer = %d, want %d", final.ReviewerID, reviewers[winnerIdx])
	}
	if final.ReviewedAt == nil {
		t.Fatal("reviewed_at should be set")
	}

	// 后到者再次处理仍不能改状态：结论保持唯一
	again, err := repo.ReviewIfPending(u.ID, 999, "rejected", "第二次结论", time.Now())
	if err != nil {
		t.Fatalf("second review: %v", err)
	}
	if again {
		t.Fatal("repeat review on processed update must not succeed")
	}
	var after model.ProjectUpdate
	if err := db.First(&after, u.ID).Error; err != nil {
		t.Fatalf("reload after repeat: %v", err)
	}
	if after.Status != final.Status || after.ReviewerID != final.ReviewerID || after.ReviewComment != final.ReviewComment {
		t.Fatal("review conclusion changed by late worker")
	}

	// 审核记录只有一份：reviewer/状态与先到者一致
	var count int64
	db.Model(&model.ProjectUpdate{}).Where("id = ?", u.ID).Count(&count)
	if count != 1 {
		t.Fatalf("update rows = %d, want 1", count)
	}
}

// TestListApprovedByProject 公开列表只返回 approved 动态。
func TestListApprovedByProject(t *testing.T) {
	db := newTestDB(t)
	repo := NewProjectUpdateRepository(db)
	p := &model.Project{OrganizationID: 1, Title: "P", Category: "education", TargetAmount: 100, Status: "approved"}
	if err := db.Create(p).Error; err != nil {
		t.Fatalf("create project: %v", err)
	}
	seed := []*model.ProjectUpdate{
		{ProjectID: p.ID, Title: "通过", Status: "approved"},
		{ProjectID: p.ID, Title: "待审核", Status: "pending"},
		{ProjectID: p.ID, Title: "驳回", Status: "rejected", ReviewComment: "原因"},
	}
	for _, u := range seed {
		if err := repo.Create(u); err != nil {
			t.Fatalf("create: %v", err)
		}
	}
	pub, err := repo.ListApprovedByProject(p.ID)
	if err != nil {
		t.Fatalf("list approved: %v", err)
	}
	if len(pub) != 1 || pub[0].Title != "通过" {
		t.Fatalf("public list = %+v, want only approved", pub)
	}
	all, err := repo.ListAllByProject(p.ID)
	if err != nil {
		t.Fatalf("list all: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("org list len = %d, want 3", len(all))
	}
}
