#!/bin/bash
# cmd/cli/analytics.go:586:3
sed -i 's/select {/ /g' cmd/cli/analytics.go
sed -i 's/case <-time.After(100 \* time.Millisecond):/<-time.After(100 \* time.Millisecond)/g' cmd/cli/analytics.go
sed -i 's/} \/\/ end select/ /g' cmd/cli/analytics.go

# cmd/cli/flags.go:447
sed -i 's/val, _ := rule\["value"\]/val := rule["value"]/g' cmd/cli/flags.go
# cmd/cli/flags.go:616
sed -i 's/value, _ := resp\["value"\]/value := resp["value"]/g' cmd/cli/flags.go

# internal/ratings/handler.go:231
sed -i 's/ErrorReportEntry{FlagKey: s.FlagKey, Evaluations: s.Evaluations, Errors: s.Errors}/ErrorReportEntry(s)/g' internal/ratings/handler.go

# internal/platform/database/postgres/ratings.go:140
sed -i 's/periodStr := "7d"//g' internal/platform/database/postgres/ratings.go

# internal/health/integrations/sentry.go:211
sed -i 's/score := 1.0//g' internal/health/integrations/sentry.go

# cmd/cli/webhooks.go:788 (strings.Title to cases.Title)
sed -i 's/strings.Title(cat)/cases.Title(language.English).String(cat)/g' cmd/cli/webhooks.go
# Need to add import "golang.org/x/text/cases" and "golang.org/x/text/language" to webhooks.go
sed -i 's|"strings"|"strings"\n\t"golang.org/x/text/cases"\n\t"golang.org/x/text/language"|g' cmd/cli/webhooks.go

# internal/deploy/handler.go:113, 137, 239 empty branches
sed -i 's/if err := h.analyticsSvc.RecordDeploymentEvent(c.Request.Context(), event); err != nil {/_ = h.analyticsSvc.RecordDeploymentEvent(c.Request.Context(), event)\n\t\t\t\/\/ /g' internal/deploy/handler.go
sed -i 's/if err := h.webhookSvc.PublishEvent(c.Request.Context(), models.EventDeploymentCreated, orgID, &d.ApplicationID, webhookData, &createdBy); err != nil {/_ = h.webhookSvc.PublishEvent(c.Request.Context(), models.EventDeploymentCreated, orgID, \&d.ApplicationID, webhookData, \&createdBy)\n\t\t\/\/ /g' internal/deploy/handler.go
