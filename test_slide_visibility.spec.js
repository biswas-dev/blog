const { test, expect } = require('@playwright/test');

test.describe('Slide Presentation Visibility Tests', () => {
  test('should display slide content without cutoff', async ({ page }) => {
    // Navigate to the slide presentation
    await page.goto('http://localhost:22222/slides/vibe-coding-to-production-mastering-cursor-ai');
    
    // Wait for the page to load completely
    await page.waitForLoadState('networkidle');
    
    // Wait for Reveal.js to initialize
    await page.waitForFunction(() => window.Reveal && window.Reveal.isReady());
    
    // Navigate to the agenda slide (slide 2)
    await page.evaluate(() => {
      window.Reveal.slide(1, 0); // Go to slide 2 (0-indexed)
    });
    
    // Wait for slide transition to complete
    await page.waitForTimeout(1000);
    
    // Take a screenshot for visual verification
    await page.screenshot({ 
      path: 'slide_agenda_visibility.png',
      fullPage: true 
    });
    
    // Check if the "Outcomes" section is fully visible (use the pill specifically)
    const outcomesSection = page.locator('.pill:has-text("Outcomes")');
    await expect(outcomesSection).toBeVisible();
    
    // Check if the "Prompt patterns" text is fully visible (not cut off)
    const promptPatternsText = page.locator('li:has-text("Prompt patterns")');
    await expect(promptPatternsText.first()).toBeVisible();

    // Check if the "Token discipline" text is fully visible (not cut off)
    const tokenDisciplineText = page.locator('li:has-text("Token discipline")');
    await expect(tokenDisciplineText.first()).toBeVisible();

    // Check if the "Ship with confidence" text is fully visible (not cut off)
    const shipWithConfidenceText = page.locator('li:has-text("Ship with confidence")');
    await expect(shipWithConfidenceText.first()).toBeVisible();
    
    // Check if the timeline "50" is visible (in SVG specifically)
    const timeline50 = page.locator('svg text:has-text("50")');
    await expect(timeline50.first()).toBeVisible();
    
    // Check if the right navigation arrow is visible
    const rightArrow = page.locator('.navigate-right');
    await expect(rightArrow).toBeVisible();
    
    // Get the bounding box of the slide content to check for clipping
    const slideSection = page.locator('.reveal .slides section').first();
    const boundingBox = await slideSection.boundingBox();
    
    // Get the viewport dimensions
    const viewport = page.viewportSize();
    
    // Check that the slide content is not cut off on the right side
    // The slide should have a max-width of 1100px and be centered
    if (boundingBox) {
      console.log('Slide bounding box:', boundingBox);
      console.log('Viewport size:', viewport);
      
      // The slide content should not extend beyond the viewport width
      expect(boundingBox.x + boundingBox.width).toBeLessThanOrEqual(viewport.width);
      
      // The slide should be centered (roughly)
      const expectedCenter = viewport.width / 2;
      const slideCenter = boundingBox.x + (boundingBox.width / 2);
      const centerOffset = Math.abs(slideCenter - expectedCenter);
      
      // Allow for some margin of error in centering
      expect(centerOffset).toBeLessThan(100);
    }
    
    // Check specific elements that were previously cut off
    const grid3Container = page.locator('.grid-3');
    await expect(grid3Container).toBeVisible();
    
    // Check that all three columns in the grid are visible
    const gridColumns = page.locator('.grid-3 > div');
    const columnCount = await gridColumns.count();
    expect(columnCount).toBe(3);
    
    // Check each column is visible and not cut off
    for (let i = 0; i < columnCount; i++) {
      const column = gridColumns.nth(i);
      await expect(column).toBeVisible();
      
      const columnBox = await column.boundingBox();
      if (columnBox) {
        // Column should not extend beyond viewport
        expect(columnBox.x + columnBox.width).toBeLessThanOrEqual(viewport.width);
      }
    }
    
    // Check the timeline SVG is fully visible
    const timelineSVG = page.locator('svg[role="img"][aria-label="Agenda timeline"]');
    await expect(timelineSVG).toBeVisible();
    
    const svgBox = await timelineSVG.boundingBox();
    if (svgBox) {
      // SVG should not extend beyond viewport
      expect(svgBox.x + svgBox.width).toBeLessThanOrEqual(viewport.width);
    }
  });
  
  test('should handle responsive design correctly', async ({ page }) => {
    // Test with a smaller viewport to check responsive behavior
    await page.setViewportSize({ width: 900, height: 600 });
    
    await page.goto('http://localhost:22222/slides/vibe-coding-to-production-mastering-cursor-ai');
    await page.waitForLoadState('networkidle');
    await page.waitForFunction(() => window.Reveal && window.Reveal.isReady());
    
    // Navigate to the agenda slide
    await page.evaluate(() => {
      window.Reveal.slide(1, 0);
    });
    
    await page.waitForTimeout(1000);
    
    // Take screenshot for responsive testing
    await page.screenshot({ 
      path: 'slide_agenda_responsive.png',
      fullPage: true 
    });
    
    // Check that content is still visible on smaller screens
    const outcomesSection = page.locator('.pill:has-text("Outcomes")');
    await expect(outcomesSection).toBeVisible();
    
    // Check that the responsive padding is applied (should be 36px instead of 84px)
    const slideSection = page.locator('.reveal .slides section').first();
    const computedStyle = await slideSection.evaluate((el) => {
      return window.getComputedStyle(el).paddingLeft;
    });
    
    // Should have reduced padding on smaller screens
    expect(computedStyle).toBe('36px');
  });
  
  test('should navigate between slides without layout issues', async ({ page }) => {
    await page.goto('http://localhost:22222/slides/vibe-coding-to-production-mastering-cursor-ai');
    await page.waitForLoadState('networkidle');
    await page.waitForFunction(() => window.Reveal && window.Reveal.isReady());
    
    // Test navigation through several slides
    for (let slideIndex = 0; slideIndex < 5; slideIndex++) {
      await page.evaluate((index) => {
        window.Reveal.slide(index, 0);
      }, slideIndex);
      
      await page.waitForTimeout(500);
      
      // Check that slide content is visible and not cut off
      const slideSection = page.locator('.reveal .slides section').nth(slideIndex);
      await expect(slideSection).toBeVisible();
      
      const boundingBox = await slideSection.boundingBox();
      if (boundingBox) {
        const viewport = page.viewportSize();
        expect(boundingBox.x + boundingBox.width).toBeLessThanOrEqual(viewport.width);
      }
    }
    
    // Take a final screenshot
    await page.screenshot({ 
      path: 'slide_navigation_test.png',
      fullPage: true 
    });
  });
});
