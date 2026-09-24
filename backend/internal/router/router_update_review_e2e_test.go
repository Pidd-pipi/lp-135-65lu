package router

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/givetrack/givetrack/internal/config"
	"github.com/givetrack/givetrack/internal/model"
	"github.com/givetrack/givetrack/internal/repository"
	"github.com/givetrack/givetrack/internal/service"
	sqlite "gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func setupE2E(t *testing.T) (*gin.Engine, *service.AuthService, *gorm.DB, *model.User, *model.User, uint) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:"+t.TempDir()+"/e2e.db?_busy_timeout=5000&_journal_mode=WAL"), &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(
		&model.User{}, &model.Organization{}, &model.Project{},
		&model.ProjectUpdate{}, &model.Donation{}, &model.AdminReview{}, &model.VolunteerService{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(8)

	admin := &model.User{Username: "admin", Email: "a@x.cn", PasswordHash: "x", Role: "admin"}
	orgUser := &model.User{Username: "org", Email: "o@x.cn", PasswordHash: "x", Role: "org"}
	if err := db.Create(admin).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(orgUser).Error; err != nil {
		t.Fatal(err)
	}
	org := &model.Organization{UserID: orgUser.ID, Name: "ORG", Status: "approved"}
	if err := db.Create(org).Error; err != nil {
		t.Fatal(err)
	}
	proj := &model.Project{OrganizationID: org.ID, Title: "P", Category: "education", TargetAmount: 1000, Status: "approved"}
	if err := db.Create(proj).Error; err != nil {
		t.Fatal(err)
	}

	userRepo := repository.NewUserRepository(db)
	orgRepo := repository.NewOrganizationRepository(db)
	projectRepo := repository.NewProjectRepository(db)
	updateRepo := repository.NewProjectUpdateRepository(db)
	donationRepo := repository.NewDonationRepository(db)
	reviewRepo := repository.NewAdminReviewRepository(db)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	authSvc := service.NewAuthService(userRepo, orgRepo, "test-secret-at-least-32-characters-long", 24, logger)
	projectSvc := service.NewProjectService(projectRepo, updateRepo, orgRepo, donationRepo, logger)
	donationSvc := service.NewDonationService(db, donationRepo, projectRepo, userRepo, logger)
	rankingSvc := service.NewRankingService(userRepo, logger)
	adminSvc := service.NewAdminService(projectRepo, updateRepo, orgRepo, reviewRepo, logger)
	cfg := &config.Config{JWTSecret: "test-secret-at-least-32-characters-long", AuthRateLimit: 1000, AuthRateWindowSecs: 60}

	engine := Setup(db, authSvc, projectSvc, donationSvc, rankingSvc, adminSvc, cfg, logger)
	return engine, authSvc, db, admin, orgUser, proj.ID
}

func do(t *testing.T, engine *gin.Engine, method, path, token string, body any) (int, map[string]any) {
	t.Helper()
	var rdr io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rdr = bytes.NewReader(b)
	}
	req := httptest.NewRequest(method, path, rdr)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	raw, _ := io.ReadAll(w.Body)
	var parsed map[string]any
	_ = json.Unmarshal(raw, &parsed)
	return w.Code, parsed
}

func TestUpdateReviewEndToEnd(t *testing.T) {
	engine, authSvc, db, admin, orgUser, projectID := setupE2E(t)
	adminTok, err := authSvc.GenerateToken(admin)
	if err != nil {
		t.Fatal(err)
	}
	orgTok, err := authSvc.GenerateToken(orgUser)
	if err != nil {
		t.Fatal(err)
	}

	// 1. 组织提交动态
	status, resp := do(t, engine, "POST", fmt.Sprintf("/api/v1/projects/%d/updates", projectID), orgTok,
		map[string]string{"title": "新进展", "content": "内容"})
	if status != http.StatusCreated {
		t.Fatalf("create status=%d resp=%v", status, resp)
	}
	updateID := uint(resp["data"].(map[string]any)["update"].(map[string]any)["id"].(float64))

	// 2. 公开详情看不到待审核动态
	status, resp = do(t, engine, "GET", fmt.Sprintf("/api/v1/projects/%d", projectID), "", nil)
	if status != 200 {
		t.Fatalf("detail status=%d", status)
	}
	ups := resp["data"].(map[string]any)["updates"].([]any)
	if len(ups) != 0 {
		t.Fatalf("public detail should hide pending update, got %d", len(ups))
	}
	status, resp = do(t, engine, "GET", fmt.Sprintf("/api/v1/projects/%d/updates", projectID), "", nil)
	if len(resp["data"].(map[string]any)["updates"].([]any)) != 0 {
		t.Fatal("public updates endpoint should hide pending")
	}

	// 3. 组织自己能看到待审核
	status, resp = do(t, engine, "GET", fmt.Sprintf("/api/v1/projects/%d/updates/mine", projectID), orgTok, nil)
	if status != 200 {
		t.Fatalf("mine status=%d", status)
	}
	mine := resp["data"].(map[string]any)["updates"].([]any)
	if len(mine) != 1 {
		t.Fatalf("org should see 1 update, got %d", len(mine))
	}
	if mine[0].(map[string]any)["status"] != "pending" {
		t.Fatalf("new update should be pending, got %v", mine[0])
	}

	// 4. 管理员待审核列表能看到
	status, resp = do(t, engine, "GET", "/api/v1/admin/updates/pending", adminTok, nil)
	if len(resp["data"].(map[string]any)["updates"].([]any)) != 1 {
		t.Fatal("admin pending list should contain the update")
	}

	// 5. 驳回不填原因 -> 400
	status, resp = do(t, engine, "POST", fmt.Sprintf("/api/v1/admin/updates/%d/review", updateID), adminTok,
		map[string]string{"status": "rejected", "comment": ""})
	if status != http.StatusBadRequest {
		t.Fatalf("reject without reason should be 400, got %d (%v)", status, resp)
	}

	// 6. 两个管理员并发审核：先到生效，后到 409 并拿到结果
	admin2 := &model.User{Username: "admin2", Email: "a2@x.cn", PasswordHash: "x", Role: "admin"}
	if err := db.Create(admin2).Error; err != nil {
		t.Fatal(err)
	}
	admin2Tok, _ := authSvc.GenerateToken(admin2)

	codes := make([]int, 2)
	bodies := make([]map[string]any, 2)
	intended := []string{"approved", "rejected"}
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i, tok := range []string{adminTok, admin2Tok} {
		wg.Add(1)
		go func(i int, tok string) {
			defer wg.Done()
			<-start
			payload := map[string]string{"status": "approved", "comment": ""}
			if i == 1 {
				payload = map[string]string{"status": "rejected", "comment": "后到者原因"}
			}
			codes[i], bodies[i] = do(t, engine, "POST", fmt.Sprintf("/api/v1/admin/updates/%d/review", updateID), tok, payload)
		}(i, tok)
	}
	time.Sleep(20 * time.Millisecond)
	close(start)
	wg.Wait()

	if codes[0] == codes[1] {
		t.Fatalf("expected one 200 and one 409, got %d and %d", codes[0], codes[1])
	}
	okCount, conflictCount := 0, 0
	winnerIdx := -1
	for i, code := range codes {
		if code == 200 {
			okCount++
			winnerIdx = i
		} else if code == 409 {
			conflictCount++
		} else {
			t.Fatalf("unexpected code %d body %v", code, bodies[i])
		}
	}
	if okCount != 1 || conflictCount != 1 {
		t.Fatalf("expected 1 success + 1 conflict, got %d/%d", okCount, conflictCount)
	}
	// 先到者结论即最终状态；后到者（409）拿到的必须是同一结果
	wantFinal := intended[winnerIdx]
	for _, b := range bodies {
		got := b["data"].(map[string]any)["update"].(map[string]any)["status"].(string)
		if got != wantFinal {
			t.Fatalf("all views must reflect first-comer result %q, got %q (bodies=%v)", wantFinal, got, bodies)
		}
	}
	finalStatus := wantFinal

	// 7. 审核结论落定后：通过则公开可见，驳回则公开不可见，状态与先到者一致
	status, resp = do(t, engine, "GET", fmt.Sprintf("/api/v1/projects/%d", projectID), "", nil)
	ups = resp["data"].(map[string]any)["updates"].([]any)
	if finalStatus == "approved" {
		if len(ups) != 1 || ups[0].(map[string]any)["status"] != "approved" {
			t.Fatalf("approved update should be public, got %v", ups)
		}
	} else {
		if len(ups) != 0 {
			t.Fatalf("rejected update should stay hidden, got %v", ups)
		}
	}

	// 8. 再审核已处理动态 -> 409，状态不变
	status, resp = do(t, engine, "POST", fmt.Sprintf("/api/v1/admin/updates/%d/review", updateID), adminTok,
		map[string]string{"status": "rejected", "comment": "重复处理"})
	if status != http.StatusConflict {
		t.Fatalf("repeat review should be 409, got %d", status)
	}
	var u model.ProjectUpdate
	db.First(&u, updateID)
	if u.Status != finalStatus {
		t.Fatalf("status changed by late review: got %s want %s", u.Status, finalStatus)
	}

	// 9. 非本人组织无权看完整动态
	org2User := &model.User{Username: "org2", Email: "o2@x.cn", PasswordHash: "x", Role: "org"}
	db.Create(org2User)
	org2 := &model.Organization{UserID: org2User.ID, Name: "ORG2", Status: "approved"}
	db.Create(org2)
	org2Tok, _ := authSvc.GenerateToken(org2User)
	status, _ = do(t, engine, "GET", fmt.Sprintf("/api/v1/projects/%d/updates/mine", projectID), org2Tok, nil)
	if status != http.StatusBadRequest && status != http.StatusForbidden {
		t.Fatalf("non-owner org should be forbidden, got %d", status)
	}

	// 10. 驳回流程：新动态驳回带原因 -> 组织能看到原因
	status, resp = do(t, engine, "POST", fmt.Sprintf("/api/v1/projects/%d/updates", projectID), orgTok,
		map[string]string{"title": "第二条", "content": "x"})
	id2 := uint(resp["data"].(map[string]any)["update"].(map[string]any)["id"].(float64))
	status, _ = do(t, engine, "POST", fmt.Sprintf("/api/v1/admin/updates/%d/review", id2), adminTok,
		map[string]string{"status": "rejected", "comment": "材料不全"})
	if status != 200 {
		t.Fatalf("reject with reason should be 200, got %d", status)
	}
	status, resp = do(t, engine, "GET", fmt.Sprintf("/api/v1/projects/%d/updates/mine", projectID), orgTok, nil)
	mine = resp["data"].(map[string]any)["updates"].([]any)
	foundRejected := false
	for _, item := range mine {
		m := item.(map[string]any)
		if uint(m["id"].(float64)) == id2 {
			if m["status"] != "rejected" || m["reviewComment"] != "材料不全" {
				t.Fatalf("org should see rejected status and reason, got %v", m)
			}
			foundRejected = true
		}
	}
	if !foundRejected {
		t.Fatal("rejected update missing in org list")
	}
	// 驳回动态不公开
	status, resp = do(t, engine, "GET", fmt.Sprintf("/api/v1/projects/%d/updates", projectID), "", nil)
	pub := resp["data"].(map[string]any)["updates"].([]any)
	wantPubLen := 0
	if finalStatus == "approved" {
		wantPubLen = 1
	}
	if len(pub) != wantPubLen {
		t.Fatalf("public list len = %d, want %d (first update %s, second rejected)", len(pub), wantPubLen, finalStatus)
	}
}
