#!/bin/bash
cd web/src

# 1. Fix types.ts
sed -i 's/: any/: unknown/g' types.ts

# 2. Fix api.ts
sed -i 's/: any/: unknown/g' api.ts
sed -i 's/<any>/<unknown>/g' api.ts

# 3. Fix dashboard.ts
sed -i 's/: any/: unknown/g' services/dashboard.ts

# 4. Fix realtime.ts
sed -i 's/: any/: unknown/g' services/realtime.ts

# 5. Fix pages
for file in pages/*.tsx; do
  sed -i 's/: any/: unknown/g' "$file"
done

# 6. Fix auth.tsx react-refresh warning
# Using eslint-disable-next-line
sed -i 's/export function useAuth(): AuthContextValue {/\/\/ eslint-disable-next-line react-refresh\/only-export-components\nexport function useAuth(): AuthContextValue {/g' auth.tsx
