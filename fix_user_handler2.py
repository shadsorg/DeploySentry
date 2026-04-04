import re

with open("internal/auth/user_handler.go", "r") as f:
    lines = f.readlines()

def comment_out(start_line, end_line):
    for i in range(start_line - 1, end_line):
        lines[i] = "// " + lines[i]

# Lines to comment out (inclusive)
# 191-207 listOrgMembers
# 209-266 inviteOrgMemberRequest & inviteOrgMember
# 268-309 changeOrgRoleRequest & changeOrgRole
# 311-334 removeOrgMember
# 336-353 listProjectMembers
# 355-387 addProjectMemberRequest & addProjectMember
# 389-425 changeProjectRoleRequest & changeProjectRole
# 427-451 removeProjectMember
ranges = [
    (191, 207),
    (209, 266),
    (268, 309),
    (311, 334),
    (336, 353),
    (355, 387),
    (389, 425),
    (427, 451)
]

for start, end in ranges:
    comment_out(start, end)

with open("internal/auth/user_handler.go", "w") as f:
    f.writelines(lines)
