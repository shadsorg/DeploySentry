package members

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/shadsorg/deploysentry/internal/models"
)

// stubMemberSvc implements just enough of Service to exercise the commit
// handler. Unimplemented methods panic via the embedded interface.
type stubMemberSvc struct {
	Service

	updateRoleCalled func(orgID, userID uuid.UUID, role models.OrgRole) error
}

func (s *stubMemberSvc) UpdateOrgMemberRole(_ context.Context, orgID, userID uuid.UUID, role models.OrgRole) error {
	if s.updateRoleCalled == nil {
		return nil
	}
	return s.updateRoleCalled(orgID, userID, role)
}

func ridPtr(id uuid.UUID) *uuid.UUID { return &id }

func TestMemberCommitHandlers_Tuples(t *testing.T) {
	tuples := MemberCommitHandlers(&stubMemberSvc{})
	if len(tuples) != 1 {
		t.Fatalf("expected 1 tuple (role_changed only per spec), got %d", len(tuples))
	}
	if tuples[0].ResourceType != "member" || tuples[0].Action != "role_changed" {
		t.Fatalf("expected member.role_changed, got %s.%s", tuples[0].ResourceType, tuples[0].Action)
	}
	if tuples[0].Handler == nil {
		t.Fatal("handler is nil")
	}
}

func TestCommitMemberRoleChanged_DispatchesOrgUserRole(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()
	var gotOrg, gotUser uuid.UUID
	var gotRole models.OrgRole
	svc := &stubMemberSvc{
		updateRoleCalled: func(o, u uuid.UUID, r models.OrgRole) error {
			gotOrg, gotUser, gotRole = o, u, r
			return nil
		},
	}
	body, _ := json.Marshal(rolePayload{Role: "admin"})
	row := &models.StagedChange{
		ResourceType: "member",
		Action:       "role_changed",
		OrgID:        orgID,
		ResourceID:   ridPtr(userID),
		NewValue:     body,
	}
	action, err := commitMemberRoleChanged(svc)(context.Background(), nil, row)
	if err != nil {
		t.Fatalf("commit error: %v", err)
	}
	if action != "member.role_changed" {
		t.Fatalf("expected member.role_changed, got %s", action)
	}
	if gotOrg != orgID || gotUser != userID || gotRole != "admin" {
		t.Fatalf("dispatch passed wrong args: org=%s user=%s role=%s", gotOrg, gotUser, gotRole)
	}
}

func TestCommitMemberRoleChanged_RequiresResourceID(t *testing.T) {
	body, _ := json.Marshal(rolePayload{Role: "admin"})
	row := &models.StagedChange{Action: "role_changed", OrgID: uuid.New(), NewValue: body}
	_, err := commitMemberRoleChanged(&stubMemberSvc{})(context.Background(), nil, row)
	if err == nil || !strings.Contains(err.Error(), "resource_id (user id) required") {
		t.Fatalf("expected resource_id error, got %v", err)
	}
}

func TestCommitMemberRoleChanged_RequiresNewValue(t *testing.T) {
	row := &models.StagedChange{Action: "role_changed", OrgID: uuid.New(), ResourceID: ridPtr(uuid.New())}
	_, err := commitMemberRoleChanged(&stubMemberSvc{})(context.Background(), nil, row)
	if err == nil || !strings.Contains(err.Error(), "new_value required") {
		t.Fatalf("expected new_value error, got %v", err)
	}
}

func TestCommitMemberRoleChanged_RequiresRole(t *testing.T) {
	body, _ := json.Marshal(rolePayload{}) // empty Role
	row := &models.StagedChange{
		Action: "role_changed", OrgID: uuid.New(),
		ResourceID: ridPtr(uuid.New()), NewValue: body,
	}
	_, err := commitMemberRoleChanged(&stubMemberSvc{})(context.Background(), nil, row)
	if err == nil || !strings.Contains(err.Error(), "role required") {
		t.Fatalf("expected role error, got %v", err)
	}
}

func TestCommitMemberRoleChanged_PropagatesServiceError(t *testing.T) {
	boom := errors.New("cannot demote last owner")
	svc := &stubMemberSvc{
		updateRoleCalled: func(uuid.UUID, uuid.UUID, models.OrgRole) error { return boom },
	}
	body, _ := json.Marshal(rolePayload{Role: "developer"})
	row := &models.StagedChange{
		Action: "role_changed", OrgID: uuid.New(),
		ResourceID: ridPtr(uuid.New()), NewValue: body,
	}
	_, err := commitMemberRoleChanged(svc)(context.Background(), nil, row)
	if err == nil || !errors.Is(err, boom) {
		t.Fatalf("expected wrapped service error, got %v", err)
	}
}
