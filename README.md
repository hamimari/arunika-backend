# Arunika Backoffice

Admin panel for the Arunika platform. Built with React 19, TypeScript, Vite, and Ant Design.

## Tech Stack

- **React 19** + **TypeScript**
- **Vite** — build tool and dev server
- **Ant Design 6** — UI component library
- **TanStack Query** — server state management
- **Zustand** — client state management
- **React Router 7** — client-side routing
- **Axios** — HTTP client

---

## Prerequisites

- Node.js >= 18
- npm >= 9
- A running instance of the [Arunika backend API](../arunika%20backend)

---

## Local Development

### 1. Install dependencies

```bash
npm install
```

### 2. Configure environment

Create a `.env.local` file in the project root:

```env
VITE_API_BASE_URL=http://localhost:8080
```

Replace the URL with the address of your local backend instance.

### 3. Start the dev server

```bash
npm run dev
```

The app will be available at `http://localhost:5173` by default.

### Other useful commands

| Command | Description |
|---------|-------------|
| `npm run build` | Type-check and compile for production |
| `npm run preview` | Serve the production build locally |
| `npm run lint` | Run ESLint |
| `npm test` | Run unit tests (Vitest) |
| `npm run test:watch` | Run tests in watch mode |

---

## Production Deployment

### Option A: Docker (recommended)

The project ships with a multi-stage `Dockerfile` that builds the app and serves it via nginx.

**Build the image:**

```bash
docker build \
  --build-arg VITE_API_BASE_URL=https://api.yourdomain.com \
  -t arunika-backoffice .
```

**Run the container:**

```bash
docker run -p 3000:80 arunika-backoffice
```

The panel will be available at `http://localhost:3000`.

> `VITE_API_BASE_URL` is baked into the static bundle at build time. Rebuild the image whenever the API URL changes.

---

### Option B: Docker Compose (full stack)

From the backend repository root, use the provided `docker-compose.yml` which includes all services (API, database, Redis, and this backoffice):

```bash
# Copy and fill in environment variables
cp .env.example .env

# Build and start all services
docker compose up --build
```

The backoffice will be served on the port defined by `BACKOFFICE_PORT` in your `.env` (default `3000`).

---

### Option C: Static hosting (Vercel, Netlify, S3, etc.)

```bash
# Set the API URL for this environment
VITE_API_BASE_URL=https://api.yourdomain.com npm run build
```

Upload the contents of the `dist/` directory to your static host. Configure the host to redirect all requests to `index.html` (SPA fallback).

Example redirect rule for nginx:

```nginx
location / {
    try_files $uri $uri/ /index.html;
}
```

---

## Project Structure

```
src/
├── api/          # Axios API clients (admin, content, packages, analytics)
├── components/   # Shared UI components (AppLayout, ContentTable, ...)
├── hooks/        # Reusable hooks (useContentPage, ...)
├── pages/        # Page components grouped by feature
│   ├── auth/
│   ├── content/  # AR cards, categories, fairy tales, dongeng pages
│   └── packages/ # Premium packages
├── store/        # Zustand stores (auth, ...)
└── App.tsx       # Router definition
```

---

## Environment Variables

| Variable | Required | Description |
|----------|----------|-------------|
| `VITE_API_BASE_URL` | Yes | Base URL of the Arunika backend API |
