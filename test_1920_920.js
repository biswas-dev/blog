const { chromium } = require('playwright');

(async () => {
  const browser = await chromium.launch();
  const context = await browser.newContext({
    viewport: { width: 1920, height: 920 } // Problematic size
  });
  const page = await context.newPage();

  await page.goto('https://anshumanbiswas.com/slides/vibe-coding-to-production-mastering-cursor-ai');
  await page.waitForLoadState('networkidle');
  await page.waitForTimeout(2000);

  await page.screenshot({ path: 'slide_1920x920.png', fullPage: false });

  console.log('Screenshot saved to slide_1920x920.png');

  await browser.close();
})();
