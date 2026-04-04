import re

with open("internal/deploy/handler.go", "r") as f:
    content = f.read()

# Replace block with proper multiline regex that is less fragile
content = re.sub(
    r'if err := h\.analyticsSvc\.RecordDeploymentEvent\(c\.Request\.Context\(\), event\); err != nil \{\s*//.*?\s*\}',
    r'_ = h.analyticsSvc.RecordDeploymentEvent(c.Request.Context(), event)',
    content
)

content = re.sub(
    r'if err := h\.webhookSvc\.PublishEvent\(c\.Request\.Context\(\), models\.EventDeploymentCreated, orgID, &d\.ApplicationID, webhookData, &createdBy\); err != nil \{\s*//.*?\s*\}',
    r'_ = h.webhookSvc.PublishEvent(c.Request.Context(), models.EventDeploymentCreated, orgID, &d.ApplicationID, webhookData, &createdBy)',
    content
)

with open("internal/deploy/handler.go", "w") as f:
    f.write(content)
