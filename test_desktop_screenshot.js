const { chromium } = require('playwright');

(async () => {
  const browser = await chromium.launch();
  const context = await browser.newContext({
    viewport: { width: 1920, height: 1080 }
  });
  const page = await context.newPage();

  await page.goto('https://anshumanbiswas.com/slides/vibe-coding-to-production-mastering-cursor-ai');
  await page.waitForLoadState('networkidle');
  await page.waitForTimeout(2000);

  await page.screenshot({ path: 'slide_desktop_view.png', fullPage: false });

  console.log('Screenshot saved to slide_desktop_view.png');

  await browser.close();
})();
