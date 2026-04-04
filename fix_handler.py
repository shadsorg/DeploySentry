import re

with open("internal/deploy/handler.go", "r") as f:
    lines = f.readlines()

for i, line in enumerate(lines):
    if "if err := h.analyticsSvc.RecordDeploymentEvent(c.Request.Context(), event); err != nil {" in line:
        lines[i] = "\t\t\t_ = h.analyticsSvc.RecordDeploymentEvent(c.Request.Context(), event)\n"
        lines[i+1] = "" # delete the closing brace
    elif "if err := h.webhookSvc.PublishEvent(c.Request.Context(), models.EventDeploymentCreated, orgID, &d.ApplicationID, webhookData, &createdBy); err != nil {" in line:
        lines[i] = "\t\t_ = h.webhookSvc.PublishEvent(c.Request.Context(), models.EventDeploymentCreated, orgID, &d.ApplicationID, webhookData, &createdBy)\n"
        lines[i+1] = "" # delete the closing brace

with open("internal/deploy/handler.go", "w") as f:
    f.writelines(lines)
