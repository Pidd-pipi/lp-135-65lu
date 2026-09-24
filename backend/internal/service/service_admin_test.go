package service

import (
	"testing"

	"github.com/givetrack/givetrack/internal/constants"
)

func TestValidateUpdateReviewInput(t *testing.T) {
	tests := []struct {
		name    string
		status  string
		comment string
		wantErr bool
	}{
		{name: "approved without comment", status: constants.UpdateApproved, comment: "", wantErr: false},
		{name: "rejected with reason", status: constants.UpdateRejected, comment: "内容不属实", wantErr: false},
		{name: "rejected without reason", status: constants.UpdateRejected, comment: "", wantErr: true},
		{name: "rejected with blank reason", status: constants.UpdateRejected, comment: "   ", wantErr: true},
		{name: "invalid status", status: "pending", comment: "", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateUpdateReviewInput(tt.status, tt.comment)
			if (err != nil) != tt.wantErr {
				t.Fatalf("validateUpdateReviewInput(%q, %q) err = %v, wantErr = %v", tt.status, tt.comment, err, tt.wantErr)
			}
		})
	}
}
