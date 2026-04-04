#!/bin/bash
# cmd/cli/analytics.go
sed -i '/select {/,/}/ c\
\t\t<-time.After(100 * time.Millisecond)' cmd/cli/analytics.go

# internal/health/integrations/sentry.go:211
sed -i 's/score := 1.0/score = 1.0/g' internal/health/integrations/sentry.go

# internal/deploy/handler.go:113, 137, 239 empty branches
# instead of replacing the if statement entirely, just add `_ = err` or similar.
# Or better, just ignore the error. The error is SA9003: empty branch.
# To fix empty branch `if err := ...; err != nil { }` we just remove the `if` and checking block, and do `_ = ...`
sed -i 's/if err := h.analyticsSvc.RecordDeploymentEvent(c.Request.Context(), event); err != nil {/_ = h.analyticsSvc.RecordDeploymentEvent(c.Request.Context(), event)/g' internal/deploy/handler.go
sed -i 's/if err := h.webhookSvc.PublishEvent(c.Request.Context(), models.EventDeploymentCreated, orgID, &d.ApplicationID, webhookData, &createdBy); err != nil {/_ = h.webhookSvc.PublishEvent(c.Request.Context(), models.EventDeploymentCreated, orgID, \&d.ApplicationID, webhookData, \&createdBy)/g' internal/deploy/handler.go

# Cleanup the trailing `}` that was left behind
sed -i '114d' internal/deploy/handler.go
sed -i '137d' internal/deploy/handler.go
sed -i '238d' internal/deploy/handler.go
