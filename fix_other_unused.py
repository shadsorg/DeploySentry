import re

# internal/rollback/controller.go
with open("internal/rollback/controller.go", "r") as f:
    content = f.read()
content = re.sub(r'var validRollbackTransitions = map\[RollbackState\]\[\]RollbackState\{.*?\n\}\n', '', content, flags=re.DOTALL)
with open("internal/rollback/controller.go", "w") as f:
    f.write(content)

# internal/analytics/handler.go
with open("internal/analytics/handler.go", "r") as f:
    content = f.read()
content = re.sub(r'func contains\(slice \[\]string, item string\) bool \{.*?\n\}\n', '', content, flags=re.DOTALL)
with open("internal/analytics/handler.go", "w") as f:
    f.write(content)

# internal/flags/targeting.go
with open("internal/flags/targeting.go", "r") as f:
    content = f.read()
content = re.sub(r'func evaluateCompoundRule\(operator CombineOperator, conditions \[\]CompoundCondition, evalCtx models\.EvaluationContext\) bool \{.*?\n\}\n', '', content, flags=re.DOTALL)
content = re.sub(r'func evaluateSingleCondition\(cond CompoundCondition, evalCtx models\.EvaluationContext\) bool \{.*?\n\}\n', '', content, flags=re.DOTALL)
with open("internal/flags/targeting.go", "w") as f:
    f.write(content)
