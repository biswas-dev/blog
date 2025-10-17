const { chromium } = require('playwright');

(async () => {
  const browser = await chromium.launch();
  const context = await browser.newContext({
    viewport: { width: 2560, height: 1440 } // Ultrawide monitor size
  });
  const page = await context.newPage();

  await page.goto('https://anshumanbiswas.com/slides/vibe-coding-to-production-mastering-cursor-ai');
  await page.waitForLoadState('networkidle');
  await page.waitForTimeout(2000);

  await page.screenshot({ path: 'slide_ultrawide_view.png', fullPage: false });

  console.log('Ultrawide screenshot saved to slide_ultrawide_view.png');

  await browser.close();
})();
