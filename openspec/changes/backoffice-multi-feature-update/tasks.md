## 1. Fix Analytics 500 Errors

- [x] 1.1 Investigate `analyticsApi.getDAU` and `analyticsApi.getNewUsers` calls — check auth headers and request params
- [x] 1.2 Create `src/pages/analytics/AnalyticsPage.tsx` with React Query hooks for DAU and new-users data
- [x] 1.3 Add error state handling in `AnalyticsPage.tsx` to display error messages on 500 responses
- [x] 1.4 Render DAU chart using Chart.js / react-chartjs-2 in `AnalyticsPage.tsx`
- [x] 1.5 Render new-users chart using Chart.js / react-chartjs-2 in `AnalyticsPage.tsx`
- [x] 1.6 Register `/analytics` route in `App.tsx`
- [x] 1.7 Add Analytics link to sidebar navigation

## 2. AR Card Image Fields

- [x] 2.1 Add `image_url` field (text input) to the AR card create/edit `Form` in `ArCardsPage.tsx`
- [x] 2.2 Add `printable_img` field (text input) to the AR card create/edit `Form` in `ArCardsPage.tsx`
- [x] 2.3 Ensure both fields are included in the form submit payload for create and update requests
- [x] 2.4 Ensure both fields are pre-populated when editing an existing AR card

## 3. AR Card Categories Management

- [x] 3.1 Create `src/pages/content/ArCardCategoriesPage.tsx` with a table listing all categories
- [x] 3.2 Implement Add Category modal/drawer with form and submit handler using `categoriesApi.create`
- [x] 3.3 Implement Edit Category modal/drawer pre-populated with existing data using `categoriesApi.update`
- [x] 3.4 Implement hide/show toggle per category row using `categoriesApi.toggleVisibility`
- [x] 3.5 Implement delete action with confirmation popover using `categoriesApi.remove`
- [x] 3.6 Register `/content/ar-card-categories` route in `App.tsx`
- [x] 3.7 Add AR Card Categories link to sidebar navigation

## 4. Show All Premium Packages

- [x] 4.1 Review `PremiumPackagesPage.tsx` query — remove or disable any active/status filter passed to `premiumPackagesApi.list`
- [x] 4.2 Add a visual indicator (badge or row style) for inactive packages in the packages table

## 5. Dongeng Page Management

- [x] 5.1 Investigate whether `/admin/content/fairy-tales/:id/pages` API endpoints exist
- [x] 5.2 If API exists, add `fairyTalePagesApi` helper functions (list, create, update, delete) in `src/api/content.ts`
- [x] 5.3 Add a Pages section to the dongeng detail view in `FairyTalesPage.tsx` showing all pages for the selected entry
- [x] 5.4 Implement Add Page form within the Pages section using the pages API
- [x] 5.5 Implement Edit Page form within the Pages section, pre-populated with existing page data
- [x] 5.6 Implement Delete Page action with confirmation within the Pages section
