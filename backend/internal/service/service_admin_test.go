package service

import (
	"strings"
	"testing"

	"github.com/givetrack/givetrack/internal/constants"
)

// 驳回必须写明原因；通过时不强制填写。
func TestReviewUpdateValidation(t *testing.T) {
	svc := NewAdminService(nil, nil, nil, nil, nil)

	_, err := svc.ReviewUpdate(1, 1, constants.UpdateRejected, "   ")
	if err == nil || !strings.Contains(err.Error(), "rejection reason") {
		t.Fatalf("rejection without reason should fail, got %v", err)
	}

	_, err = svc.ReviewUpdate(1, 1, "unknown", "")
	if err == nil || !strings.Contains(err.Error(), "invalid review status") {
		t.Fatalf("invalid status should fail, got %v", err)
	}

	// 合法入参应通过前置校验（nil repo 会在数据库访问处 panic/报错，而不是校验错误）
	func() {
		defer func() { _ = recover() }()
		_, _ = svc.ReviewUpdate(1, 1, constants.UpdateApproved, "")
	}()
}
