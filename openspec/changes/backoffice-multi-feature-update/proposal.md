## Why

The backoffice has several broken or incomplete features: analytics endpoints return 500 errors, AR card management is missing image field support, there is no AR card category management UI, inactive premium packages are hidden from admins, and the dongeng (fairy tales) menu lacks page-level content management. These gaps reduce operational efficiency and block content management workflows.

## What Changes

- **Fix** 500 errors on `GET /admin/analytics/new-users` and `GET /admin/analytics/dau` endpoints (or their frontend consumers)
- **Add** `image_url` and `printable_img` fields to the AR card create/update form
- **Add** a new AR Card Categories menu with full CRUD (add, update, hide/show, delete)
- **Fix** premium packages list to display all packages regardless of active/inactive status
- **Add** page management to the Dongeng (Fairy Tales) menu — create, update, and delete individual pages within a fairy tale

## Capabilities

### New Capabilities

- `analytics-dashboard`: View DAU and new-user analytics charts without 500 errors
- `ar-card-image-fields`: Manage `image_url` and `printable_img` on AR cards in the admin UI
- `ar-card-categories`: Full CRUD management of AR card categories (add, update, hide, delete)
- `dongeng-pages`: Add, update, and delete pages within an existing dongeng (fairy tale) entry

### Modified Capabilities

- `content-management`: AR card form extended with new image fields; dongeng detail extended with page management
- `premium-packages`: Package list now shows all packages including inactive ones

## Impact

- `src/api/analytics.ts` — investigate and fix `getDAU` / `getNewUsers` request or error handling
- `src/api/content.ts` — may need new API methods for AR card categories pages and dongeng pages
- `src/pages/content/ArCardsPage.tsx` — add `image_url` and `printable_img` to create/edit modal
- `src/pages/content/CategoriesPage.tsx` — may be reused or extended for AR card categories
- `src/pages/content/FairyTalesPage.tsx` — add nested page management UI
- `src/pages/packages/PremiumPackagesPage.tsx` — remove active-only filter from list query
- New route/page may be needed for AR card category management
