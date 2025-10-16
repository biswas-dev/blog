const { chromium } = require('playwright');

(async () => {
  const browser = await chromium.launch();
  const context = await browser.newContext({
    viewport: { width: 1920, height: 1080 }
  });
  const page = await context.newPage();

  await page.goto('https://anshumanbiswas.com/slides/vibe-coding-to-production-mastering-cursor-ai');
  await page.waitForLoadState('networkidle');

  // Wait for Reveal to be ready
  await page.waitForFunction(() => window.Reveal && window.Reveal.isReady());

  // Navigate to slide 2 (agenda)
  await page.evaluate(() => window.Reveal.slide(1, 0));
  await page.waitForTimeout(1000);

  await page.screenshot({ path: 'slide_agenda_desktop.png', fullPage: false });

  console.log('Agenda slide screenshot saved to slide_agenda_desktop.png');

  await browser.close();
})();
