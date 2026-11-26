# Stub of the test writer. It builds an image with no website or web server.
FROM node:26.13.0-alpine AS site
FROM site AS runtime
