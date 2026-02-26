# Vue 3 + Vite

This template should help get you started developing with Vue 3 in Vite. The template uses Vue 3 `<script setup>` SFCs, check out the [script setup docs](https://v3.vuejs.org/api/sfc-script-setup.html#sfc-script-setup) to learn more.

Learn more about IDE Support for Vue in the [Vue Docs Scaling up Guide](https://vuejs.org/guide/scaling-up/tooling.html#ide-support).

## Dev notes for this repo

- Frontend uses same-origin API paths by default: `/api`.
- `web/vite.config.js` proxies `/api` (and `/analysis`) to `http://localhost:19527` for local dev.
- Optional override: set `VITE_API_BASE="http://127.0.0.1:19527/api"` before `npm run dev`.
