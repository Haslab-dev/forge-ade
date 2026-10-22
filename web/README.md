# ForgeADE Web Landing Page

The official marketing and documentation landing page for [ForgeADE](https://github.com/Haslab-dev/forge-ade).

Designed with a high-density, refined developer aesthetic (monochrome graphite, deep zinc surfaces, zero AI-slop neon gradients) powered by **Vite**, **React 19**, and **Tailwind CSS v4**.

---

## Local Development

```bash
cd web
bun install
bun run dev
```

The dev server will spin up at `http://localhost:5173`.

---

## Production Build

```bash
cd web
bun run build
```

The production assets and optimized bundle will be generated in `web/dist`.

---

## Deploy to Cloudflare Pages

### Option 1: Automatic Deployment via Cloudflare Dashboard (Recommended)

1. Go to the [Cloudflare Dashboard](https://dash.cloudflare.com/) > **Workers & Pages** > **Create application** > **Pages** > **Connect to Git**.
2. Select repository: `Haslab-dev/forge-ade`.
3. Set **Build settings**:
   - **Framework preset**: `Vite`
   - **Root directory**: `web`
   - **Build command**: `bun run build`
   - **Build output directory**: `dist`
4. Click **Save and Deploy**.

### Option 2: Direct CLI Deployment via Wrangler

You can deploy directly from your terminal using Wrangler:

```bash
cd web
bun run build
bunx wrangler pages deploy dist --project-name=forge-ade
```
