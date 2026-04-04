import re

with open("internal/deploy/handler.go", "r") as f:
    content = f.read()

# For RecordDeploymentEvent empty block
# It looks like:
# if err := h.analyticsSvc.RecordDeploymentEvent(c.Request.Context(), event); err != nil {
#     // Log error but don't fail the request
# }
content = re.sub(
    r'if err := h\.analyticsSvc\.RecordDeploymentEvent\(c\.Request\.Context\(\), event\); err != nil \{\s*// Log error but don\'t fail the request\s*\}',
    r'_ = h.analyticsSvc.RecordDeploymentEvent(c.Request.Context(), event)',
    content
)

# For webhookSvc.PublishEvent empty block
content = re.sub(
    r'if err := h\.webhookSvc\.PublishEvent\(c\.Request\.Context\(\), models\.EventDeploymentCreated, orgID, &d\.ApplicationID, webhookData, &createdBy\); err != nil \{\s*// Log error but don\'t fail the request\s*\}',
    r'_ = h.webhookSvc.PublishEvent(c.Request.Context(), models.EventDeploymentCreated, orgID, &d.ApplicationID, webhookData, &createdBy)',
    content
)

with open("internal/deploy/handler.go", "w") as f:
    f.write(content)
