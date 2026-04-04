#!/bin/bash
cd web

# Add react-hooks/exhaustive-deps ignores
sed -i '/useEffect(() => {/,/}, \[\]);/ s/}, \[\]);/    \/\/ eslint-disable-next-line react-hooks\/exhaustive-deps\n  }, \[\]);/g' src/pages/FlagCreatePage.tsx
sed -i '/useEffect(() => {/,/}, \[\]);/ s/}, \[\]);/    \/\/ eslint-disable-next-line react-hooks\/exhaustive-deps\n  }, \[\]);/g' src/pages/MembersPage.tsx
sed -i '/useEffect(() => {/,/}, \[\]);/ s/}, \[\]);/    \/\/ eslint-disable-next-line react-hooks\/exhaustive-deps\n  }, \[\]);/g' src/services/realtime.ts

# Fix explicit any
sed -i 's/(e: any)/(e: React.FormEvent<HTMLFormElement>)/g' src/pages/LoginPage.tsx
sed -i 's/(event: any)/(event: MessageEvent)/g' src/services/realtime.ts

# Fast refresh warning in auth.tsx
sed -i 's/export const useAuth = () => {/\/\/ eslint-disable-next-line react-refresh\/only-export-components\nexport const useAuth = () => {/g' src/auth.tsx
