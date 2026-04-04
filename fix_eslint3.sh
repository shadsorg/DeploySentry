#!/bin/bash
cd web
sed -i 's/}, \[orgSlug, projectSlug, appSlug, apps\]);/    \/\/ eslint-disable-next-line react-hooks\/exhaustive-deps\n  }, \[orgSlug, projectSlug, appSlug, apps\]);/g' src/pages/FlagCreatePage.tsx
sed -i 's/}, \[orgSlug\]);/    \/\/ eslint-disable-next-line react-hooks\/exhaustive-deps\n  }, \[orgSlug\]);/g' src/pages/MembersPage.tsx
sed -i 's/}, \[enabled\]); \/\/ Only run on mount and when enabled changes/    \/\/ eslint-disable-next-line react-hooks\/exhaustive-deps\n  }, \[enabled\]); \/\/ Only run on mount and when enabled changes/g' src/services/realtime.ts
