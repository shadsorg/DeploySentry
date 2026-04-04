#!/bin/bash
cd web/src

# 1. LoginPage
sed -i "s/(location.state as any)/(location.state as { from?: { pathname: string } })/g" pages/LoginPage.tsx

# 2. realtime.ts
sed -i "s/Record<string, any>/Record<string, unknown>/g" services/realtime.ts

# 3. Suppress exhaustive-deps for FlagCreatePage.tsx
sed -i 's/  }, \[orgSlug, projectSlug, appSlug, apps\]);/\/\/ eslint-disable-next-line react-hooks\/exhaustive-deps\n  }, \[orgSlug, projectSlug, appSlug, apps\]);/g' pages/FlagCreatePage.tsx

# 4. Suppress exhaustive-deps for MembersPage.tsx
sed -i 's/  }, \[orgSlug\]);/  \/\/ eslint-disable-next-line react-hooks\/exhaustive-deps\n  }, \[orgSlug\]);/g' pages/MembersPage.tsx

# 5. Suppress exhaustive-deps for realtime.ts
sed -i 's/  }, \[enabled\]); \/\/ Only run on mount and when enabled changes/  \/\/ eslint-disable-next-line react-hooks\/exhaustive-deps\n  }, \[enabled\]); \/\/ Only run on mount and when enabled changes/g' services/realtime.ts
