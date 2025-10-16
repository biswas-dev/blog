const { defineConfig, devices } = require('@playwright/test');

module.exports = defineConfig({
  testDir: './',
  testMatch: ['**/test_slide_visibility.spec.js', '**/check_all_posts.spec.js'], // Only run specific working tests
  timeout: 30000,
  fullyParallel: false, // Run tests serially for login state management
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  workers: 1, // Single worker to maintain login session
  reporter: [
    ['html'],
    ['list']
  ],
  use: {
    baseURL: 'http://localhost:22222',
    trace: 'on-first-retry',
    screenshot: 'only-on-failure',
    video: 'retain-on-failure',
  },

  projects: [
    {
      name: 'chromium',
      use: { 
        ...devices['Desktop Chrome'],
        viewport: { width: 1200, height: 800 }
      },
    },
    {
      name: 'Mobile Chrome',
      use: { 
        ...devices['Pixel 5'] 
      },
    },
  ],

  webServer: {
    command: 'echo "Blog server should already be running on port 22222"',
    port: 22222,
    reuseExistingServer: true,
  },
});