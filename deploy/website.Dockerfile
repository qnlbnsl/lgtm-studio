# syntax=docker/dockerfile:1
# The website image. The stage site builds the website with the server address "/", so the page
# calls the server at its own address. The stage runtime serves the built files with the web
# server rules of deploy/Caddyfile, as an ordinary user. Build it from the repository root:
#   docker build -f deploy/website.Dockerfile -t lgtm-web:local .
# The allow-list next to this file keeps the state folder and every other path out of the build.

FROM node:26.13.0-alpine@sha256:8996bb5821d62a8fd9bb27c1cb07462f3fdce6e47d640c6a881ead0b064ce216 AS site
# Node 26 ships without Corepack, so pnpm is installed with npm, at the version of package.json.
RUN npm install --global pnpm@12.16.1
WORKDIR /repo
COPY . /repo
RUN --mount=type=cache,target=/pnpm/store \
    pnpm install --frozen-lockfile --store-dir /pnpm/store --filter @lgtm/frontend...
RUN VITE_LGTM_SERVER=/ pnpm --filter @lgtm/frontend build

FROM caddy:2.11.9-alpine@sha256:003a97808e781aa9f7e75cc2f43d34d461099031faf5e20faefb9f61126836f0 AS runtime
COPY --from=site /repo/src/frontend/dist /srv/site
COPY deploy/Caddyfile /etc/caddy/Caddyfile
USER 10003:10003
ENV XDG_DATA_HOME=/tmp XDG_CONFIG_HOME=/tmp
CMD ["caddy", "run", "--config", "/etc/caddy/Caddyfile"]
