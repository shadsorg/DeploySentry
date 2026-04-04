#!/bin/bash

# internal/deploy/handler.go empty branches
# We replace the entire if block with a simple assignment.
sed -i 's/if err := h.analyticsSvc.RecordDeploymentEvent(c.Request.Context(), event); err != nil {/_ = h.analyticsSvc.RecordDeploymentEvent(c.Request.Context(), event)/g' internal/deploy/handler.go
sed -i 's/if err := h.webhookSvc.PublishEvent(c.Request.Context(), models.EventDeploymentCreated, orgID, &d.ApplicationID, webhookData, &createdBy); err != nil {/_ = h.webhookSvc.PublishEvent(c.Request.Context(), models.EventDeploymentCreated, orgID, \&d.ApplicationID, webhookData, \&createdBy)/g' internal/deploy/handler.go
# Now we need to remove the trailing '}' on the next line.
# A simpler approach: use python to safely replace the multi-line block.
