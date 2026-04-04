#!/bin/bash
git restore internal/deploy/handler.go
sed -i 's/if err := h.analyticsSvc.RecordDeploymentEvent(c.Request.Context(), event); err != nil {/_ = h.analyticsSvc.RecordDeploymentEvent(c.Request.Context(), event)\n\t\t\t\/\//g' internal/deploy/handler.go
sed -i 's/if err := h.webhookSvc.PublishEvent(c.Request.Context(), models.EventDeploymentCreated, orgID, &d.ApplicationID, webhookData, &createdBy); err != nil {/_ = h.webhookSvc.PublishEvent(c.Request.Context(), models.EventDeploymentCreated, orgID, \&d.ApplicationID, webhookData, \&createdBy)\n\t\t\/\//g' internal/deploy/handler.go
