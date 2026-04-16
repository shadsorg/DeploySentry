package models

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/stretchr/testify/assert"
)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func validUser() User {
	return User{
		ID:           uuid.New(),
		Email:        "alice@example.com",
		Name:         "Alice",
		AuthProvider: AuthProviderGitHub,
	}
}

func validDeployment() Deployment {
	return Deployment{
		ID:            uuid.New(),
		ApplicationID: uuid.New(),
		EnvironmentID: uuid.New(),
		Artifact:      "myapp:latest",
		Version:       "1.0.0",
		CreatedBy:     uuid.New(),
		Strategy:      DeployStrategyCanary,
		Status:        DeployStatusPending,
	}
}

func uuidPtr(u uuid.UUID) *uuid.UUID { return &u }

func validFeatureFlag() FeatureFlag {
	return FeatureFlag{
		ID:            uuid.New(),
		ProjectID:     uuid.New(),
		EnvironmentID: uuidPtr(uuid.New()),
		Key:           "enable-dark-mode",
		Name:          "Enable Dark Mode",
		FlagType:      FlagTypeBoolean,
	}
}

func validRelease() Release {
	return Release{
		ID:            uuid.New(),
		ApplicationID: uuid.New(),
		Name:          "Initial Release",
		Status:        ReleaseDraft,
	}
}

func validOrganization() Organization {
	return Organization{
		ID:      uuid.New(),
		Name:    "Acme Corp",
		Slug:    "acme-corp",
		OwnerID: uuid.New(),
	}
}

func validOrgMember() OrgMember {
	return OrgMember{
		ID:     uuid.New(),
		OrgID:  uuid.New(),
		UserID: uuid.New(),
		Role:   OrgRoleMember,
	}
}

func validProject() Project {
	return Project{
		ID:   uuid.New(),
		OrgID: uuid.New(),
		Name: "My Project",
		Slug: "my-project",
	}
}

func validEnvironment() Environment {
	return Environment{
		ID:            uuid.New(),
		ApplicationID: uuid.New(),
		Name:          "production",
		Slug:          "production",
	}
}

func validAPIKey() APIKey {
	return APIKey{
		ID:             uuid.New(),
		OrgID:          uuid.New(),
		EnvironmentIDs: []uuid.UUID{},
		Name:           "CI Key",
		Scopes:         []APIKeyScope{APIKeyScopeReadFlags},
		CreatedBy:      uuid.New(),
	}
}

func validWebhook() Webhook {
	pid := uuid.New()
	return Webhook{
		ID:        uuid.New(),
		OrgID:     uuid.New(),
		ProjectID: &pid,
		URL:       "https://example.com/webhook",
		Events:    pq.StringArray{"deployment.completed"},
	}
}

func intPtr(v int) *int { return &v }

// ---------------------------------------------------------------------------
// 1. User.Validate()
// ---------------------------------------------------------------------------

func TestUserValidate(t *testing.T) {
	tests := []struct {
		name    string
		modify  func(*User)
		wantErr string
	}{
		{
			name:    "valid user",
			modify:  func(u *User) {},
			wantErr: "",
		},
		{
			name:    "empty email",
			modify:  func(u *User) { u.Email = "" },
			wantErr: "email is required",
		},
		{
			name:    "invalid email without @",
			modify:  func(u *User) { u.Email = "not-an-email" },
			wantErr: "email must be a valid email address",
		},
		{
			name:    "empty name",
			modify:  func(u *User) { u.Name = "" },
			wantErr: "name is required",
		},
		{
			name:    "name too long",
			modify:  func(u *User) { u.Name = strings.Repeat("a", 201) },
			wantErr: "name must be 200 characters or fewer",
		},
		{
			name:    "valid auth provider github",
			modify:  func(u *User) { u.AuthProvider = AuthProviderGitHub },
			wantErr: "",
		},
		{
			name:    "valid auth provider google",
			modify:  func(u *User) { u.AuthProvider = AuthProviderGoogle },
			wantErr: "",
		},
		{
			name:    "valid auth provider email",
			modify:  func(u *User) { u.AuthProvider = AuthProviderEmail },
			wantErr: "",
		},
		{
			name:    "invalid auth provider",
			modify:  func(u *User) { u.AuthProvider = "ldap" },
			wantErr: "unsupported auth provider",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			u := validUser()
			tc.modify(&u)
			err := u.Validate()
			if tc.wantErr == "" {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, tc.wantErr)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// 2. Deployment.Validate()
// ---------------------------------------------------------------------------

func TestDeploymentValidate(t *testing.T) {
	tests := []struct {
		name    string
		modify  func(*Deployment)
		wantErr string
	}{
		{
			name:    "valid deployment",
			modify:  func(d *Deployment) {},
			wantErr: "",
		},
		{
			name:    "missing application_id",
			modify:  func(d *Deployment) { d.ApplicationID = uuid.Nil },
			wantErr: "application_id is required",
		},
		{
			name:    "missing environment_id",
			modify:  func(d *Deployment) { d.EnvironmentID = uuid.Nil },
			wantErr: "environment_id is required",
		},
		{
			name:    "empty artifact",
			modify:  func(d *Deployment) { d.Artifact = "" },
			wantErr: "artifact is required",
		},
		{
			name:    "empty version",
			modify:  func(d *Deployment) { d.Version = "" },
			wantErr: "version is required",
		},
		{
			name:    "missing created_by",
			modify:  func(d *Deployment) { d.CreatedBy = uuid.Nil },
			wantErr: "created_by is required",
		},
		{
			name:    "valid strategy canary",
			modify:  func(d *Deployment) { d.Strategy = DeployStrategyCanary },
			wantErr: "",
		},
		{
			name:    "valid strategy blue_green",
			modify:  func(d *Deployment) { d.Strategy = DeployStrategyBlueGreen },
			wantErr: "",
		},
		{
			name:    "valid strategy rolling",
			modify:  func(d *Deployment) { d.Strategy = DeployStrategyRolling },
			wantErr: "",
		},
		{
			name:    "invalid strategy",
			modify:  func(d *Deployment) { d.Strategy = "recreate" },
			wantErr: "unsupported deploy strategy",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			d := validDeployment()
			tc.modify(&d)
			err := d.Validate()
			if tc.wantErr == "" {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, tc.wantErr)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// 3. Deployment.ValidateTransition()
// ---------------------------------------------------------------------------

func TestDeploymentValidateTransition(t *testing.T) {
	// All valid transitions.
	validCases := []struct {
		from DeployStatus
		to   DeployStatus
	}{
		// pending ->
		{DeployStatusPending, DeployStatusRunning},
		{DeployStatusPending, DeployStatusCancelled},
		// running ->
		{DeployStatusRunning, DeployStatusPaused},
		{DeployStatusRunning, DeployStatusPromoting},
		{DeployStatusRunning, DeployStatusCompleted},
		{DeployStatusRunning, DeployStatusFailed},
		{DeployStatusRunning, DeployStatusRolledBack},
		{DeployStatusRunning, DeployStatusCancelled},
		// paused ->
		{DeployStatusPaused, DeployStatusRunning},
		{DeployStatusPaused, DeployStatusRolledBack},
		{DeployStatusPaused, DeployStatusCancelled},
		// promoting ->
		{DeployStatusPromoting, DeployStatusCompleted},
		{DeployStatusPromoting, DeployStatusFailed},
		{DeployStatusPromoting, DeployStatusRolledBack},
	}

	for _, tc := range validCases {
		t.Run(string(tc.from)+"->"+string(tc.to), func(t *testing.T) {
			d := Deployment{Status: tc.from}
			err := d.ValidateTransition(tc.to)
			assert.NoError(t, err)
		})
	}

	// Terminal states should not allow any transitions.
	terminalStates := []DeployStatus{
		DeployStatusCompleted,
		DeployStatusFailed,
		DeployStatusRolledBack,
		DeployStatusCancelled,
	}
	allStatuses := []DeployStatus{
		DeployStatusPending, DeployStatusRunning, DeployStatusPaused,
		DeployStatusPromoting, DeployStatusCompleted, DeployStatusFailed,
		DeployStatusRolledBack, DeployStatusCancelled,
	}

	for _, terminal := range terminalStates {
		for _, target := range allStatuses {
			t.Run(string(terminal)+"->"+string(target)+"_rejected", func(t *testing.T) {
				d := Deployment{Status: terminal}
				err := d.ValidateTransition(target)
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "invalid status transition")
			})
		}
	}

	// Invalid transitions from non-terminal states.
	invalidCases := []struct {
		from DeployStatus
		to   DeployStatus
	}{
		{DeployStatusPending, DeployStatusCompleted},
		{DeployStatusPending, DeployStatusFailed},
		{DeployStatusPending, DeployStatusPaused},
		{DeployStatusPending, DeployStatusPromoting},
		{DeployStatusPending, DeployStatusRolledBack},
		{DeployStatusPaused, DeployStatusCompleted},
		{DeployStatusPaused, DeployStatusPromoting},
		{DeployStatusPromoting, DeployStatusPaused},
		{DeployStatusPromoting, DeployStatusRunning},
		{DeployStatusPromoting, DeployStatusCancelled},
	}

	for _, tc := range invalidCases {
		t.Run(string(tc.from)+"->"+string(tc.to)+"_invalid", func(t *testing.T) {
			d := Deployment{Status: tc.from}
			err := d.ValidateTransition(tc.to)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "invalid status transition")
		})
	}

	// Unknown current status.
	t.Run("unknown_current_status", func(t *testing.T) {
		d := Deployment{Status: "unknown_state"}
		err := d.ValidateTransition(DeployStatusRunning)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unknown current status")
	})
}

// ---------------------------------------------------------------------------
// 4. Deployment.TransitionTo()
// ---------------------------------------------------------------------------

func TestDeploymentTransitionTo(t *testing.T) {
	t.Run("pending to running sets StartedAt", func(t *testing.T) {
		d := validDeployment()
		d.Status = DeployStatusPending
		assert.Nil(t, d.StartedAt)

		err := d.TransitionTo(DeployStatusRunning)
		assert.NoError(t, err)
		assert.Equal(t, DeployStatusRunning, d.Status)
		assert.NotNil(t, d.StartedAt)
	})

	t.Run("running to completed sets CompletedAt", func(t *testing.T) {
		d := validDeployment()
		d.Status = DeployStatusRunning
		now := time.Now().UTC()
		d.StartedAt = &now
		assert.Nil(t, d.CompletedAt)

		err := d.TransitionTo(DeployStatusCompleted)
		assert.NoError(t, err)
		assert.Equal(t, DeployStatusCompleted, d.Status)
		assert.NotNil(t, d.CompletedAt)
	})

	t.Run("running to failed sets CompletedAt", func(t *testing.T) {
		d := validDeployment()
		d.Status = DeployStatusRunning
		now := time.Now().UTC()
		d.StartedAt = &now

		err := d.TransitionTo(DeployStatusFailed)
		assert.NoError(t, err)
		assert.Equal(t, DeployStatusFailed, d.Status)
		assert.NotNil(t, d.CompletedAt)
	})

	t.Run("running to rolled_back sets CompletedAt", func(t *testing.T) {
		d := validDeployment()
		d.Status = DeployStatusRunning
		now := time.Now().UTC()
		d.StartedAt = &now

		err := d.TransitionTo(DeployStatusRolledBack)
		assert.NoError(t, err)
		assert.Equal(t, DeployStatusRolledBack, d.Status)
		assert.NotNil(t, d.CompletedAt)
	})

	t.Run("pending to cancelled sets CompletedAt", func(t *testing.T) {
		d := validDeployment()
		d.Status = DeployStatusPending

		err := d.TransitionTo(DeployStatusCancelled)
		assert.NoError(t, err)
		assert.Equal(t, DeployStatusCancelled, d.Status)
		assert.NotNil(t, d.CompletedAt)
	})

	t.Run("StartedAt is not overwritten on resume", func(t *testing.T) {
		d := validDeployment()
		d.Status = DeployStatusPending

		// First transition to running.
		err := d.TransitionTo(DeployStatusRunning)
		assert.NoError(t, err)
		firstStart := *d.StartedAt

		// Pause then resume.
		err = d.TransitionTo(DeployStatusPaused)
		assert.NoError(t, err)
		err = d.TransitionTo(DeployStatusRunning)
		assert.NoError(t, err)

		// StartedAt should still be the original time.
		assert.Equal(t, firstStart, *d.StartedAt)
	})

	t.Run("invalid transition returns error", func(t *testing.T) {
		d := validDeployment()
		d.Status = DeployStatusPending

		err := d.TransitionTo(DeployStatusCompleted)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid status transition")
		// Status should remain unchanged on error.
		assert.Equal(t, DeployStatusPending, d.Status)
	})
}

// ---------------------------------------------------------------------------
// 5. Deployment.IsTerminal()
// ---------------------------------------------------------------------------

func TestDeploymentIsTerminal(t *testing.T) {
	tests := []struct {
		status   DeployStatus
		terminal bool
	}{
		{DeployStatusPending, false},
		{DeployStatusRunning, false},
		{DeployStatusPaused, false},
		{DeployStatusPromoting, false},
		{DeployStatusCompleted, true},
		{DeployStatusFailed, true},
		{DeployStatusRolledBack, true},
		{DeployStatusCancelled, true},
	}

	for _, tc := range tests {
		t.Run(string(tc.status), func(t *testing.T) {
			d := Deployment{Status: tc.status}
			assert.Equal(t, tc.terminal, d.IsTerminal())
		})
	}
}

// ---------------------------------------------------------------------------
// 6. FeatureFlag.Validate()
// ---------------------------------------------------------------------------

func TestFeatureFlagValidate(t *testing.T) {
	tests := []struct {
		name    string
		modify  func(*FeatureFlag)
		wantErr string
	}{
		{
			name:    "valid flag",
			modify:  func(f *FeatureFlag) {},
			wantErr: "",
		},
		{
			name:    "missing project_id",
			modify:  func(f *FeatureFlag) { f.ProjectID = uuid.Nil },
			wantErr: "project_id is required",
		},
		{
			name:    "empty key",
			modify:  func(f *FeatureFlag) { f.Key = "" },
			wantErr: "flag key is required",
		},
		{
			name:    "key too long",
			modify:  func(f *FeatureFlag) { f.Key = strings.Repeat("k", 201) },
			wantErr: "flag key must be 200 characters or fewer",
		},
		{
			name:    "empty name",
			modify:  func(f *FeatureFlag) { f.Name = "" },
			wantErr: "flag name is required",
		},
		{
			name:    "valid type boolean",
			modify:  func(f *FeatureFlag) { f.FlagType = FlagTypeBoolean },
			wantErr: "",
		},
		{
			name:    "valid type string",
			modify:  func(f *FeatureFlag) { f.FlagType = FlagTypeString },
			wantErr: "",
		},
		{
			name:    "valid type integer",
			modify:  func(f *FeatureFlag) { f.FlagType = FlagTypeInteger },
			wantErr: "",
		},
		{
			name:    "valid type json",
			modify:  func(f *FeatureFlag) { f.FlagType = FlagTypeJSON },
			wantErr: "",
		},
		{
			name:    "invalid flag type",
			modify:  func(f *FeatureFlag) { f.FlagType = "float" },
			wantErr: "unsupported flag type",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := validFeatureFlag()
			tc.modify(&f)
			err := f.Validate()
			if tc.wantErr == "" {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, tc.wantErr)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// 7. TargetingRule.Validate()
// ---------------------------------------------------------------------------

func TestTargetingRuleValidate(t *testing.T) {
	flagID := uuid.New()
	segID := uuid.New()
	now := time.Now().UTC()
	later := now.Add(time.Hour)

	tests := []struct {
		name    string
		rule    TargetingRule
		wantErr string
	}{
		{
			name: "missing flag_id",
			rule: TargetingRule{
				FlagID:     uuid.Nil,
				RuleType:   RuleTypePercentage,
				Percentage: intPtr(50),
			},
			wantErr: "flag_id is required",
		},
		{
			name: "percentage rule without percentage",
			rule: TargetingRule{
				FlagID:   flagID,
				RuleType: RuleTypePercentage,
			},
			wantErr: "percentage is required for percentage rules",
		},
		{
			name: "percentage below 0",
			rule: TargetingRule{
				FlagID:     flagID,
				RuleType:   RuleTypePercentage,
				Percentage: intPtr(-1),
			},
			wantErr: "percentage must be between 0 and 100",
		},
		{
			name: "percentage above 100",
			rule: TargetingRule{
				FlagID:     flagID,
				RuleType:   RuleTypePercentage,
				Percentage: intPtr(101),
			},
			wantErr: "percentage must be between 0 and 100",
		},
		{
			name: "valid percentage rule at 0",
			rule: TargetingRule{
				FlagID:     flagID,
				RuleType:   RuleTypePercentage,
				Percentage: intPtr(0),
			},
			wantErr: "",
		},
		{
			name: "valid percentage rule at 100",
			rule: TargetingRule{
				FlagID:     flagID,
				RuleType:   RuleTypePercentage,
				Percentage: intPtr(100),
			},
			wantErr: "",
		},
		{
			name: "user_target without target_values",
			rule: TargetingRule{
				FlagID:   flagID,
				RuleType: RuleTypeUserTarget,
			},
			wantErr: "target_values required for user_target rules",
		},
		{
			name: "valid user_target",
			rule: TargetingRule{
				FlagID:       flagID,
				RuleType:     RuleTypeUserTarget,
				TargetValues: []string{"user-1", "user-2"},
			},
			wantErr: "",
		},
		{
			name: "attribute without attribute field",
			rule: TargetingRule{
				FlagID:   flagID,
				RuleType: RuleTypeAttribute,
				Operator: "eq",
			},
			wantErr: "attribute is required for attribute rules",
		},
		{
			name: "attribute without operator",
			rule: TargetingRule{
				FlagID:    flagID,
				RuleType:  RuleTypeAttribute,
				Attribute: "country",
			},
			wantErr: "operator is required for attribute rules",
		},
		{
			name: "valid attribute rule",
			rule: TargetingRule{
				FlagID:    flagID,
				RuleType:  RuleTypeAttribute,
				Attribute: "country",
				Operator:  "eq",
			},
			wantErr: "",
		},
		{
			name: "segment without segment_id",
			rule: TargetingRule{
				FlagID:   flagID,
				RuleType: RuleTypeSegment,
			},
			wantErr: "segment_id is required for segment rules",
		},
		{
			name: "valid segment rule",
			rule: TargetingRule{
				FlagID:    flagID,
				RuleType:  RuleTypeSegment,
				SegmentID: &segID,
			},
			wantErr: "",
		},
		{
			name: "schedule without times",
			rule: TargetingRule{
				FlagID:   flagID,
				RuleType: RuleTypeSchedule,
			},
			wantErr: "start_time and end_time are required for schedule rules",
		},
		{
			name: "schedule with only start_time",
			rule: TargetingRule{
				FlagID:    flagID,
				RuleType:  RuleTypeSchedule,
				StartTime: &now,
			},
			wantErr: "start_time and end_time are required for schedule rules",
		},
		{
			name: "schedule with only end_time",
			rule: TargetingRule{
				FlagID:   flagID,
				RuleType: RuleTypeSchedule,
				EndTime:  &later,
			},
			wantErr: "start_time and end_time are required for schedule rules",
		},
		{
			name: "valid schedule rule",
			rule: TargetingRule{
				FlagID:    flagID,
				RuleType:  RuleTypeSchedule,
				StartTime: &now,
				EndTime:   &later,
			},
			wantErr: "",
		},
		{
			name: "invalid rule type",
			rule: TargetingRule{
				FlagID:   flagID,
				RuleType: "unknown",
			},
			wantErr: "unsupported rule type",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.rule.Validate()
			if tc.wantErr == "" {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, tc.wantErr)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// 8. Release.Validate()
// ---------------------------------------------------------------------------

func TestReleaseValidate(t *testing.T) {
	tests := []struct {
		name    string
		modify  func(*Release)
		wantErr string
	}{
		{
			name:    "valid release",
			modify:  func(r *Release) {},
			wantErr: "",
		},
		{
			name:    "missing application_id",
			modify:  func(r *Release) { r.ApplicationID = uuid.Nil },
			wantErr: "application_id is required",
		},
		{
			name:    "missing name",
			modify:  func(r *Release) { r.Name = "" },
			wantErr: "name is required",
		},
		{
			name: "sticky without header",
			modify: func(r *Release) {
				r.SessionSticky = true
				r.StickyHeader = ""
			},
			wantErr: "sticky_header is required when session_sticky is true",
		},
		{
			name: "sticky with header",
			modify: func(r *Release) {
				r.SessionSticky = true
				r.StickyHeader = "X-Release-Id"
			},
			wantErr: "",
		},
		{
			name:    "traffic_percent below 0",
			modify:  func(r *Release) { r.TrafficPercent = -1 },
			wantErr: "traffic_percent must be between 0 and 100",
		},
		{
			name:    "traffic_percent above 100",
			modify:  func(r *Release) { r.TrafficPercent = 101 },
			wantErr: "traffic_percent must be between 0 and 100",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := validRelease()
			tc.modify(&r)
			err := r.Validate()
			if tc.wantErr == "" {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, tc.wantErr)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// 9. Release.TransitionTo()
// ---------------------------------------------------------------------------

func TestReleaseTransitionTo(t *testing.T) {
	// All valid transitions.
	validCases := []struct {
		from ReleaseStatus
		to   ReleaseStatus
	}{
		{ReleaseDraft, ReleaseRollingOut},
		{ReleaseRollingOut, ReleasePaused},
		{ReleaseRollingOut, ReleaseCompleted},
		{ReleaseRollingOut, ReleaseRolledBack},
		{ReleasePaused, ReleaseRollingOut},
		{ReleasePaused, ReleaseRolledBack},
	}

	for _, tc := range validCases {
		t.Run(string(tc.from)+"->"+string(tc.to), func(t *testing.T) {
			r := Release{Status: tc.from, ApplicationID: uuid.New(), Name: "test"}
			err := r.TransitionTo(tc.to)
			assert.NoError(t, err)
			assert.Equal(t, tc.to, r.Status)
		})
	}

	// Terminal states should not allow any transitions.
	terminalStates := []ReleaseStatus{
		ReleaseCompleted,
		ReleaseRolledBack,
	}
	allReleaseStatuses := []ReleaseStatus{
		ReleaseDraft, ReleaseRollingOut, ReleasePaused,
		ReleaseCompleted, ReleaseRolledBack,
	}
	for _, terminal := range terminalStates {
		for _, target := range allReleaseStatuses {
			t.Run(string(terminal)+"->"+string(target)+"_rejected", func(t *testing.T) {
				r := Release{Status: terminal}
				err := r.TransitionTo(target)
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "no transitions from terminal status")
			})
		}
	}

	// Invalid transitions from non-terminal states.
	invalidCases := []struct {
		from ReleaseStatus
		to   ReleaseStatus
	}{
		{ReleaseDraft, ReleaseCompleted},
		{ReleaseDraft, ReleasePaused},
		{ReleaseDraft, ReleaseRolledBack},
		{ReleaseRollingOut, ReleaseDraft},
		{ReleasePaused, ReleaseDraft},
		{ReleasePaused, ReleaseCompleted},
	}

	for _, tc := range invalidCases {
		t.Run(string(tc.from)+"->"+string(tc.to)+"_invalid", func(t *testing.T) {
			r := Release{Status: tc.from}
			err := r.TransitionTo(tc.to)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "invalid transition")
		})
	}
}

// ---------------------------------------------------------------------------
// 10. Organization.Validate()
// ---------------------------------------------------------------------------

func TestOrganizationValidate(t *testing.T) {
	tests := []struct {
		name    string
		modify  func(*Organization)
		wantErr string
	}{
		{
			name:    "valid organization",
			modify:  func(o *Organization) {},
			wantErr: "",
		},
		{
			name:    "empty name",
			modify:  func(o *Organization) { o.Name = "" },
			wantErr: "organization name is required",
		},
		{
			name:    "name too long",
			modify:  func(o *Organization) { o.Name = strings.Repeat("x", 101) },
			wantErr: "organization name must be 100 characters or fewer",
		},
		{
			name:    "empty slug",
			modify:  func(o *Organization) { o.Slug = "" },
			wantErr: "organization slug is required",
		},
		{
			name:    "slug too long",
			modify:  func(o *Organization) { o.Slug = strings.Repeat("s", 61) },
			wantErr: "organization slug must be 60 characters or fewer",
		},
		{
			name:    "missing owner_id",
			modify:  func(o *Organization) { o.OwnerID = uuid.Nil },
			wantErr: "organization owner_id is required",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			o := validOrganization()
			tc.modify(&o)
			err := o.Validate()
			if tc.wantErr == "" {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, tc.wantErr)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// 11. ValidRole()
// ---------------------------------------------------------------------------

func TestValidRole(t *testing.T) {
	tests := []struct {
		role  OrgRole
		valid bool
	}{
		{OrgRoleOwner, true},
		{OrgRoleAdmin, true},
		{OrgRoleMember, true},
		{OrgRoleViewer, true},
		{"superadmin", false},
		{"", false},
	}

	for _, tc := range tests {
		t.Run(string(tc.role), func(t *testing.T) {
			assert.Equal(t, tc.valid, ValidRole(tc.role))
		})
	}
}

// ---------------------------------------------------------------------------
// 12. OrgMember.Validate()
// ---------------------------------------------------------------------------

func TestOrgMemberValidate(t *testing.T) {
	tests := []struct {
		name    string
		modify  func(*OrgMember)
		wantErr string
	}{
		{
			name:    "valid member",
			modify:  func(m *OrgMember) {},
			wantErr: "",
		},
		{
			name:    "missing org_id",
			modify:  func(m *OrgMember) { m.OrgID = uuid.Nil },
			wantErr: "org_id is required",
		},
		{
			name:    "missing user_id",
			modify:  func(m *OrgMember) { m.UserID = uuid.Nil },
			wantErr: "user_id is required",
		},
		{
			name:    "invalid role",
			modify:  func(m *OrgMember) { m.Role = "superadmin" },
			wantErr: "invalid organization role",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := validOrgMember()
			tc.modify(&m)
			err := m.Validate()
			if tc.wantErr == "" {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, tc.wantErr)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// 13. Project.Validate()
// ---------------------------------------------------------------------------

func TestProjectValidate(t *testing.T) {
	tests := []struct {
		name    string
		modify  func(*Project)
		wantErr string
	}{
		{
			name:    "valid project",
			modify:  func(p *Project) {},
			wantErr: "",
		},
		{
			name:    "missing org_id",
			modify:  func(p *Project) { p.OrgID = uuid.Nil },
			wantErr: "org_id is required",
		},
		{
			name:    "empty name",
			modify:  func(p *Project) { p.Name = "" },
			wantErr: "project name is required",
		},
		{
			name:    "name too long",
			modify:  func(p *Project) { p.Name = strings.Repeat("n", 101) },
			wantErr: "project name must be 100 characters or fewer",
		},
		{
			name:    "empty slug",
			modify:  func(p *Project) { p.Slug = "" },
			wantErr: "project slug is required",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := validProject()
			tc.modify(&p)
			err := p.Validate()
			if tc.wantErr == "" {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, tc.wantErr)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// 14. Environment.Validate()
// ---------------------------------------------------------------------------

func TestEnvironmentValidate(t *testing.T) {
	tests := []struct {
		name    string
		modify  func(*Environment)
		wantErr string
	}{
		{
			name:    "valid environment",
			modify:  func(e *Environment) {},
			wantErr: "",
		},
		{
			name:    "missing application_id",
			modify:  func(e *Environment) { e.ApplicationID = uuid.Nil },
			wantErr: "application_id is required",
		},
		{
			name:    "empty name",
			modify:  func(e *Environment) { e.Name = "" },
			wantErr: "environment name is required",
		},
		{
			name:    "empty slug",
			modify:  func(e *Environment) { e.Slug = "" },
			wantErr: "environment slug is required",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			e := validEnvironment()
			tc.modify(&e)
			err := e.Validate()
			if tc.wantErr == "" {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, tc.wantErr)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// 15. APIKey.Validate()
// ---------------------------------------------------------------------------

func TestAPIKeyValidate(t *testing.T) {
	tests := []struct {
		name    string
		modify  func(*APIKey)
		wantErr string
	}{
		{
			name:    "valid key",
			modify:  func(k *APIKey) {},
			wantErr: "",
		},
		{
			name:    "nil org_id",
			modify:  func(k *APIKey) { k.OrgID = uuid.Nil },
			wantErr: "org_id is required",
		},
		{
			name:    "empty name",
			modify:  func(k *APIKey) { k.Name = "" },
			wantErr: "name is required",
		},
		{
			name:    "no scopes",
			modify:  func(k *APIKey) { k.Scopes = nil },
			wantErr: "at least one scope is required",
		},
		{
			name:    "empty scopes slice",
			modify:  func(k *APIKey) { k.Scopes = []APIKeyScope{} },
			wantErr: "at least one scope is required",
		},
		{
			name:    "invalid scope",
			modify:  func(k *APIKey) { k.Scopes = []APIKeyScope{"invalid:scope"} },
			wantErr: "invalid scope: invalid:scope",
		},
		{
			name: "mix of valid and invalid scopes",
			modify: func(k *APIKey) {
				k.Scopes = []APIKeyScope{APIKeyScopeReadFlags, "bad"}
			},
			wantErr: "invalid scope: bad",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			k := validAPIKey()
			tc.modify(&k)
			err := k.Validate()
			if tc.wantErr == "" {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, tc.wantErr)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// 16. APIKey.IsExpired()
// ---------------------------------------------------------------------------

func TestAPIKeyIsExpired(t *testing.T) {
	t.Run("no expiry is not expired", func(t *testing.T) {
		k := validAPIKey()
		k.ExpiresAt = nil
		assert.False(t, k.IsExpired())
	})

	t.Run("future expiry is not expired", func(t *testing.T) {
		k := validAPIKey()
		future := time.Now().UTC().Add(24 * time.Hour)
		k.ExpiresAt = &future
		assert.False(t, k.IsExpired())
	})

	t.Run("past expiry is expired", func(t *testing.T) {
		k := validAPIKey()
		past := time.Now().UTC().Add(-24 * time.Hour)
		k.ExpiresAt = &past
		assert.True(t, k.IsExpired())
	})
}

// ---------------------------------------------------------------------------
// 17. APIKey.IsRevoked()
// ---------------------------------------------------------------------------

func TestAPIKeyIsRevoked(t *testing.T) {
	t.Run("no revoked_at is not revoked", func(t *testing.T) {
		k := validAPIKey()
		k.RevokedAt = nil
		assert.False(t, k.IsRevoked())
	})

	t.Run("has revoked_at is revoked", func(t *testing.T) {
		k := validAPIKey()
		now := time.Now().UTC()
		k.RevokedAt = &now
		assert.True(t, k.IsRevoked())
	})
}

// ---------------------------------------------------------------------------
// 18. APIKey.HasScope()
// ---------------------------------------------------------------------------

func TestAPIKeyHasScope(t *testing.T) {
	t.Run("has exact scope", func(t *testing.T) {
		k := validAPIKey()
		k.Scopes = []APIKeyScope{APIKeyScopeReadFlags, APIKeyScopeWriteDeploys}
		assert.True(t, k.HasScope(APIKeyScopeReadFlags))
		assert.True(t, k.HasScope(APIKeyScopeWriteDeploys))
	})

	t.Run("does not have scope", func(t *testing.T) {
		k := validAPIKey()
		k.Scopes = []APIKeyScope{APIKeyScopeReadFlags}
		assert.False(t, k.HasScope(APIKeyScopeWriteDeploys))
		assert.False(t, k.HasScope(APIKeyScopeAdmin))
	})

	t.Run("admin scope grants all", func(t *testing.T) {
		k := validAPIKey()
		k.Scopes = []APIKeyScope{APIKeyScopeAdmin}
		assert.True(t, k.HasScope(APIKeyScopeReadFlags))
		assert.True(t, k.HasScope(APIKeyScopeWriteFlags))
		assert.True(t, k.HasScope(APIKeyScopeReadDeploys))
		assert.True(t, k.HasScope(APIKeyScopeWriteDeploys))
		assert.True(t, k.HasScope(APIKeyScopeReadReleases))
		assert.True(t, k.HasScope(APIKeyScopeWriteReleases))
		assert.True(t, k.HasScope(APIKeyScopeAdmin))
	})
}

// ---------------------------------------------------------------------------
// 19. Webhook.Validate()
// ---------------------------------------------------------------------------

func TestWebhookValidate(t *testing.T) {
	tests := []struct {
		name    string
		modify  func(*Webhook)
		wantErr string
	}{
		{
			name:    "valid webhook",
			modify:  func(w *Webhook) {},
			wantErr: "",
		},
		{
			name:    "missing org_id",
			modify:  func(w *Webhook) { w.OrgID = uuid.Nil },
			wantErr: "org_id is required",
		},
		{
			name:   "missing project_id",
			modify: func(w *Webhook) { nilID := uuid.Nil; w.ProjectID = &nilID },
			wantErr: "project_id is required",
		},
		{
			name:    "empty url",
			modify:  func(w *Webhook) { w.URL = "" },
			wantErr: "url is required",
		},
		{
			name:    "no event types",
			modify:  func(w *Webhook) { w.Events = nil },
			wantErr: "at least one event type is required",
		},
		{
			name:    "empty event types slice",
			modify:  func(w *Webhook) { w.Events = pq.StringArray{} },
			wantErr: "at least one event type is required",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w := validWebhook()
			tc.modify(&w)
			err := w.Validate()
			if tc.wantErr == "" {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, tc.wantErr)
			}
		})
	}
}
