const { defineConfig, devices } = require("@playwright/test");

module.exports = defineConfig({
  testDir: ".",
  testMatch: "*.spec.js",
  fullyParallel: false,
  forbidOnly: true,
  retries: 1,
  workers: 1,
  timeout: 30_000,
  reporter: [["line"]],
  use: {
    baseURL: process.env.RJS_ADMIN_UI_URL || "http://host.docker.internal:8223",
    trace: "retain-on-failure",
    screenshot: "only-on-failure"
  },
  projects: [
    { name: "chromium", use: { ...devices["Desktop Chrome"] } },
    { name: "firefox", use: { ...devices["Desktop Firefox"] } }
  ]
});
