FROM mcr.microsoft.com/playwright:v1.55.0-noble
WORKDIR /tests
COPY package.json package-lock.json ./
RUN npm ci --no-audit --no-fund
COPY playwright.config.js admin-ui.spec.js ./
ENTRYPOINT ["npm", "test", "--"]
