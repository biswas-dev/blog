const { test, expect } = require('@playwright/test');

test('Check all posts on homepage', async ({ page }) => {
  await page.goto('http://localhost:22222');
  await page.waitForLoadState('networkidle');
  
  // Count articles
  const articles = page.locator('article.blog-post');
  const articleCount = await articles.count();
  
  console.log(`Total articles displayed: ${articleCount}`);
  
  // Get all post titles and links
  for (let i = 0; i < articleCount; i++) {
    const article = articles.nth(i);
    const titleElement = article.locator('h2.blog-post-title a');
    const title = await titleElement.textContent();
    const href = await titleElement.getAttribute('href');
    const dateElement = article.locator('time');
    const date = await dateElement.getAttribute('datetime');
    
    console.log(`Post ${i + 1}: "${title?.trim()}" -> ${href} (${date})`);
  }
  
  // Check if our 3 technical posts are present
  const technicalPosts = [
    'Optimizing Cloud Resource Allocation with Machine Learning',
    'Building Resilient Distributed Systems', 
    'The Future of Cloud Middleware Performance'
  ];
  
  const pageText = await page.textContent('body');
  
  for (const postTitle of technicalPosts) {
    const isVisible = pageText.includes(postTitle);
    console.log(`"${postTitle}" visible: ${isVisible}`);
  }
  
  // Check if links are working
  const firstLink = articles.nth(0).locator('h2.blog-post-title a');
  const firstHref = await firstLink.getAttribute('href');
  console.log(`First post link: ${firstHref}`);
  
  expect(articleCount).toBeGreaterThanOrEqual(1); // Should show at least 1 post
});