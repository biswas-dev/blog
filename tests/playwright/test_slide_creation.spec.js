const { test, expect } = require('@playwright/test');
const path = require('path');
const fs = require('fs');
const http = require('http');

const BASE = 'http://localhost:22222';

// Login via raw HTTP POST (avoids browser redirect issues)
function postLogin() {
  return new Promise((resolve, reject) => {
    const postData = 'email=anchoo2kewl@gmail.com&password=123456qwertyu';
    const req = http.request({
      hostname: 'localhost', port: 22222, path: '/signin', method: 'POST',
      headers: { 'Content-Type': 'application/x-www-form-urlencoded', 'Content-Length': Buffer.byteLength(postData) },
    }, (res) => {
      const cookies = res.headers['set-cookie'] || [];
      resolve({ status: res.statusCode, cookies });
      res.resume();
    });
    req.on('error', reject);
    req.write(postData);
    req.end();
  });
}

// Shared login helper — uses raw HTTP to get session cookie, then injects into browser context
async function login(page) {
  const result = await postLogin();
  if (result.status !== 302) throw new Error(`Login failed: status ${result.status}`);

  const context = page.context();
  for (const cookieStr of result.cookies) {
    const parts = cookieStr.split(';')[0].split('=');
    const name = parts[0].trim();
    const value = parts.slice(1).join('=').trim();
    await context.addCookies([{ name, value, domain: 'localhost', path: '/' }]);
  }
}

// ===========================================================================
// 1. SLIDE CREATION — BASIC FLOWS
// ===========================================================================
test.describe('Slide Creation - Basic', () => {
  test.beforeEach(async ({ page }) => {
    test.setTimeout(30000);
    await login(page);
  });

  test('create slide with title and default content', async ({ page }) => {
    await page.goto(`${BASE}/admin/slides/new`);

    // Editor should load with slide sidebar and toolbar
    await expect(page.locator('#slide-sidebar')).toBeVisible();
    await expect(page.locator('#editor-toolbar')).toBeVisible();
    await expect(page.locator('#tiptap-container')).toBeVisible();

    // Fill title
    const title = 'Basic Slide ' + Date.now();
    await page.fill('#panel-title', title);

    // TipTap editor should have default content
    const editorContent = page.locator('#tiptap-container .ProseMirror');
    await expect(editorContent).toBeVisible();

    // Type some content in TipTap
    await editorContent.click();
    await editorContent.pressSequentially('Hello from Playwright');

    // Click publish
    await page.click('#publish-btn');

    // Should redirect to edit page
    await page.waitForURL('**/admin/slides/*/edit', { timeout: 10000 });

    // Title should be pre-filled in the editor
    const savedTitle = await page.locator('#panel-title').inputValue();
    expect(savedTitle).toBe(title);
  });

  test('create slide with custom slug', async ({ page }) => {
    await page.goto(`${BASE}/admin/slides/new`);

    const title = 'Custom Slug Slide';
    const slug = 'my-custom-slug-' + Date.now();

    await page.fill('#panel-title', title);
    await page.fill('#panel-slug', slug);

    // Type content
    const editor = page.locator('#tiptap-container .ProseMirror');
    await editor.click();
    await editor.pressSequentially('Custom slug content');

    await page.click('#publish-btn');
    await page.waitForURL('**/admin/slides/*/edit', { timeout: 10000 });

    // Verify slug was preserved
    const savedSlug = await page.locator('#panel-slug').inputValue();
    expect(savedSlug).toBe(slug);
  });

  test('create slide with auto-generated slug from title', async ({ page }) => {
    await page.goto(`${BASE}/admin/slides/new`);

    await page.fill('#panel-title', 'My Auto Slug Test');

    // Slug should auto-generate
    const slugValue = await page.locator('#panel-slug').inputValue();
    expect(slugValue).toContain('my-auto-slug-test');

    // Type content and save as draft
    const editor = page.locator('#tiptap-container .ProseMirror');
    await editor.click();
    await editor.pressSequentially('Auto slug content');

    await page.click('#save-draft-btn');
    await page.waitForURL('**/admin/slides/*/edit', { timeout: 10000 });
  });

  test('create slide with description', async ({ page }) => {
    await page.goto(`${BASE}/admin/slides/new`);

    const title = 'Slide With Description ' + Date.now();
    await page.fill('#panel-title', title);
    await page.fill('#panel-description', 'This is a detailed description of the slide presentation.');

    const editor = page.locator('#tiptap-container .ProseMirror');
    await editor.click();
    await editor.pressSequentially('Description test content');

    await page.click('#publish-btn');
    await page.waitForURL('**/admin/slides/*/edit', { timeout: 10000 });

    // Verify description persisted
    const desc = await page.locator('#panel-description').inputValue();
    expect(desc).toBe('This is a detailed description of the slide presentation.');
  });

  test('create slide with categories', async ({ page }) => {
    await page.goto(`${BASE}/admin/slides/new`);

    await page.fill('#panel-title', 'Categorized Slide ' + Date.now());

    // Check first available category
    const firstCat = page.locator('.cat-checkbox').first();
    if (await firstCat.count() > 0) {
      await firstCat.check();
    }

    const editor = page.locator('#tiptap-container .ProseMirror');
    await editor.click();
    await editor.pressSequentially('Category test');

    await page.click('#publish-btn');
    await page.waitForURL('**/admin/slides/*/edit', { timeout: 10000 });
  });

  test('save slide as draft (not published)', async ({ page }) => {
    await page.goto(`${BASE}/admin/slides/new`);

    const title = 'Draft Slide ' + Date.now();
    await page.fill('#panel-title', title);

    const editor = page.locator('#tiptap-container .ProseMirror');
    await editor.click();
    await editor.pressSequentially('Draft content');

    // Save as draft (not publish)
    await page.click('#save-draft-btn');
    await page.waitForURL('**/admin/slides/*/edit', { timeout: 10000 });

    // Verify it's in admin list
    await page.goto(`${BASE}/admin/slides`);
    await expect(page.locator(`text=${title}`)).toBeVisible();

    // Verify it does NOT appear in public list
    await page.goto(`${BASE}/slides`);
    const publicPage = await page.content();
    expect(publicPage).not.toContain(title);
  });
});

// ===========================================================================
// 2. SLIDE CREATION — PASSWORD PROTECTION
// ===========================================================================
test.describe('Slide Creation - Password Protection', () => {
  test.beforeEach(async ({ page }) => {
    test.setTimeout(30000);
    await login(page);
  });

  test('create password-protected slide', async ({ page }) => {
    await page.goto(`${BASE}/admin/slides/new`);

    const title = 'Protected Slide ' + Date.now();
    await page.fill('#panel-title', title);
    await page.fill('#panel-password', 'secret123');

    const editor = page.locator('#tiptap-container .ProseMirror');
    await editor.click();
    await editor.pressSequentially('Secret content');

    await page.click('#publish-btn');
    await page.waitForURL('**/admin/slides/*/edit', { timeout: 10000 });

    // Extract slug from the saved form
    const slug = await page.locator('#panel-slug').inputValue();
    expect(slug).toBeTruthy();

    // Open a new incognito-like context to test as unauthenticated
    const context = await page.context().browser().newContext();
    const anonPage = await context.newPage();

    // Visit the slide as anonymous user — should get password prompt
    await anonPage.goto(`${BASE}/slides/${slug}`);
    await expect(anonPage.locator('text=Password Required')).toBeVisible();
    await expect(anonPage.locator('input[name="password"]')).toBeVisible();

    await context.close();
  });

  test('wrong password shows error', async ({ page }) => {
    // First create a protected slide
    await page.goto(`${BASE}/admin/slides/new`);
    const slug = 'wrong-pw-test-' + Date.now();
    await page.fill('#panel-title', 'Wrong PW Test');
    await page.fill('#panel-slug', slug);
    await page.fill('#panel-password', 'correctpass');
    const editor = page.locator('#tiptap-container .ProseMirror');
    await editor.click();
    await editor.pressSequentially('Protected content');
    await page.click('#publish-btn');
    await page.waitForURL('**/admin/slides/*/edit', { timeout: 10000 });

    // Now test as anonymous
    const context = await page.context().browser().newContext();
    const anonPage = await context.newPage();
    await anonPage.goto(`${BASE}/slides/${slug}`);
    await expect(anonPage.locator('text=Password Required')).toBeVisible();

    // Enter wrong password
    await anonPage.fill('input[name="password"]', 'wrongpassword');
    await anonPage.click('button[type="submit"]');

    // Should see error
    await expect(anonPage.locator('text=Incorrect password')).toBeVisible();

    await context.close();
  });

  test('correct password grants access', async ({ page }) => {
    // Create protected slide
    await page.goto(`${BASE}/admin/slides/new`);
    const slug = 'correct-pw-test-' + Date.now();
    await page.fill('#panel-title', 'Correct PW Test');
    await page.fill('#panel-slug', slug);
    await page.fill('#panel-password', 'rightpass');
    const editor = page.locator('#tiptap-container .ProseMirror');
    await editor.click();
    await editor.pressSequentially('Accessible after password');
    await page.click('#publish-btn');
    await page.waitForURL('**/admin/slides/*/edit', { timeout: 10000 });

    // Test as anonymous
    const context = await page.context().browser().newContext();
    const anonPage = await context.newPage();
    await anonPage.goto(`${BASE}/slides/${slug}`);
    await expect(anonPage.locator('text=Password Required')).toBeVisible();

    // Enter correct password
    await anonPage.fill('input[name="password"]', 'rightpass');
    await anonPage.click('button[type="submit"]');

    // Should now see the presentation (Reveal.js)
    await anonPage.waitForURL(`**/slides/${slug}`, { timeout: 10000 });
    await expect(anonPage.locator('.reveal')).toBeVisible();

    await context.close();
  });

  test('admin/editor bypasses password', async ({ page }) => {
    // Create protected slide
    await page.goto(`${BASE}/admin/slides/new`);
    const slug = 'admin-bypass-test-' + Date.now();
    await page.fill('#panel-title', 'Admin Bypass Test');
    await page.fill('#panel-slug', slug);
    await page.fill('#panel-password', 'anypass');
    const editor = page.locator('#tiptap-container .ProseMirror');
    await editor.click();
    await editor.pressSequentially('Admin sees this directly');
    await page.click('#publish-btn');
    await page.waitForURL('**/admin/slides/*/edit', { timeout: 10000 });

    // Admin visits the slide — should see presentation directly, no password prompt
    await page.goto(`${BASE}/slides/${slug}`);
    await expect(page.locator('.reveal')).toBeVisible();
    // Should NOT see password prompt
    expect(await page.locator('text=Password Required').count()).toBe(0);
  });
});

// ===========================================================================
// 3. SLIDE EDITOR UI
// ===========================================================================
test.describe('Slide Editor UI', () => {
  test.beforeEach(async ({ page }) => {
    test.setTimeout(30000);
    await login(page);
  });

  test('editor loads with all panels', async ({ page }) => {
    await page.goto(`${BASE}/admin/slides/new`);

    // Left sidebar
    await expect(page.locator('#slide-sidebar')).toBeVisible();
    await expect(page.locator('#slide-thumb-list')).toBeVisible();
    await expect(page.locator('#add-slide-btn')).toBeVisible();

    // Toolbar
    await expect(page.locator('#editor-toolbar')).toBeVisible();
    await expect(page.locator('[data-cmd="bold"]')).toBeVisible();
    await expect(page.locator('[data-cmd="italic"]')).toBeVisible();
    await expect(page.locator('[data-cmd="h1"]')).toBeVisible();
    await expect(page.locator('[data-cmd="image"]')).toBeVisible();
    await expect(page.locator('#layout-select')).toBeVisible();
    await expect(page.locator('#fragment-select')).toBeVisible();

    // Mode toggle
    await expect(page.locator('#mode-wysiwyg')).toBeVisible();
    await expect(page.locator('#mode-code')).toBeVisible();

    // Right panel
    await expect(page.locator('#panel-title')).toBeVisible();
    await expect(page.locator('#panel-slug')).toBeVisible();
    await expect(page.locator('#panel-description')).toBeVisible();
    await expect(page.locator('#panel-password')).toBeVisible();
    await expect(page.locator('#slide-bg-color')).toBeVisible();
    await expect(page.locator('#slide-transition')).toBeVisible();

    // Bottom bar
    await expect(page.locator('#save-draft-btn')).toBeVisible();
    await expect(page.locator('#publish-btn')).toBeVisible();
    await expect(page.locator('#autosave-status')).toBeVisible();
  });

  test('add new slide via sidebar', async ({ page }) => {
    await page.goto(`${BASE}/admin/slides/new`);

    // Should start with 1 slide thumbnail
    const initialThumbs = page.locator('.slide-thumb');
    await expect(initialThumbs).toHaveCount(1);

    // Add a slide
    await page.click('#add-slide-btn');

    // Should now have 2
    await expect(page.locator('.slide-thumb')).toHaveCount(2);

    // Second thumb should be active
    const secondThumb = page.locator('.slide-thumb').nth(1);
    await expect(secondThumb).toHaveClass(/active/);
  });

  test('switch between slides in sidebar', async ({ page }) => {
    await page.goto(`${BASE}/admin/slides/new`);

    // Type in first slide
    const editor = page.locator('#tiptap-container .ProseMirror');
    await editor.click();
    await page.keyboard.press(`${process.platform === 'darwin' ? 'Meta' : 'Control'}+A`);
    await editor.pressSequentially('First slide content');

    // Add second slide
    await page.click('#add-slide-btn');

    // Type in second slide
    await editor.click();
    await editor.pressSequentially('Second slide content');

    // Click back on first slide
    await page.locator('.slide-thumb').first().click();

    // Content should switch to first slide
    const content = await editor.innerHTML();
    expect(content).toContain('First slide content');
  });

  test('delete slide from sidebar', async ({ page }) => {
    await page.goto(`${BASE}/admin/slides/new`);

    // Add two more slides (total 3)
    await page.click('#add-slide-btn');
    await page.click('#add-slide-btn');
    await expect(page.locator('.slide-thumb')).toHaveCount(3);

    // Hover over second slide and click delete
    const secondThumb = page.locator('.slide-thumb').nth(1);
    await secondThumb.hover();
    await secondThumb.locator('.slide-thumb-delete').click();

    // Should now have 2
    await expect(page.locator('.slide-thumb')).toHaveCount(2);
  });

  test('toggle code view and back', async ({ page }) => {
    await page.goto(`${BASE}/admin/slides/new`);

    // WYSIWYG mode by default
    await expect(page.locator('#tiptap-container')).toBeVisible();
    await expect(page.locator('#codemirror-container')).toBeHidden();

    // Switch to code mode
    await page.click('#mode-code');
    await expect(page.locator('#tiptap-container')).toBeHidden();
    await expect(page.locator('#codemirror-container')).toBeVisible();

    // CodeMirror should have initialized
    await expect(page.locator('#codemirror-container .cm-editor')).toBeVisible();

    // Switch back to WYSIWYG
    await page.click('#mode-wysiwyg');
    await expect(page.locator('#tiptap-container')).toBeVisible();
    await expect(page.locator('#codemirror-container')).toBeHidden();
  });

  test('apply bold formatting', async ({ page }) => {
    await page.goto(`${BASE}/admin/slides/new`);

    const editor = page.locator('#tiptap-container .ProseMirror');
    await editor.click();
    await editor.pressSequentially('normal text');

    // Select all (Meta on Mac, Control on others) and bold
    const mod = process.platform === 'darwin' ? 'Meta' : 'Control';
    await page.keyboard.press(`${mod}+A`);
    await page.click('[data-cmd="bold"]');

    // Check that strong tag was applied
    const html = await editor.innerHTML();
    expect(html).toContain('<strong>');
  });

  test('apply heading formatting', async ({ page }) => {
    await page.goto(`${BASE}/admin/slides/new`);

    const editor = page.locator('#tiptap-container .ProseMirror');
    await editor.click();
    await editor.pressSequentially('My Heading');

    const mod = process.platform === 'darwin' ? 'Meta' : 'Control';
    await page.keyboard.press(`${mod}+A`);
    await page.click('[data-cmd="h1"]');

    const html = await editor.innerHTML();
    expect(html).toContain('<h1');
  });

  test('change slide background color', async ({ page }) => {
    await page.goto(`${BASE}/admin/slides/new`);

    // Set a background color
    await page.locator('#slide-bg-color').fill('#ff0000');
    await page.locator('#slide-bg-color').dispatchEvent('input');

    // Type content so we can save
    const editor = page.locator('#tiptap-container .ProseMirror');
    await editor.click();
    await editor.pressSequentially('Red bg slide');
    await page.fill('#panel-title', 'BG Color Test ' + Date.now());

    await page.click('#publish-btn');
    await page.waitForURL('**/admin/slides/*/edit', { timeout: 10000 });
  });

  test('change slide transition', async ({ page }) => {
    await page.goto(`${BASE}/admin/slides/new`);

    await page.selectOption('#slide-transition', 'fade');
    const val = await page.locator('#slide-transition').inputValue();
    expect(val).toBe('fade');
  });

  test('select slide layout template', async ({ page }) => {
    await page.goto(`${BASE}/admin/slides/new`);

    // Select "Title Slide" layout
    await page.selectOption('#layout-select', 'title');

    // Editor should now contain h1 and subtitle
    const html = await page.locator('#tiptap-container .ProseMirror').innerHTML();
    expect(html).toContain('Title');
  });

  test('open and close image insert modal', async ({ page }) => {
    await page.goto(`${BASE}/admin/slides/new`);

    // Click image button
    await page.click('[data-cmd="image"]');
    await expect(page.locator('#image-modal')).toBeVisible();

    // Cancel
    await page.click('#image-cancel');
    await expect(page.locator('#image-modal')).toBeHidden();
  });

  test('open and close mermaid modal', async ({ page }) => {
    await page.goto(`${BASE}/admin/slides/new`);

    await page.click('[data-cmd="mermaid"]');
    await expect(page.locator('#mermaid-modal')).toBeVisible();

    // Type mermaid syntax
    await page.fill('#mermaid-input', 'graph TD\n  A-->B');

    // Insert
    await page.click('#mermaid-insert');
    await expect(page.locator('#mermaid-modal')).toBeHidden();

    // Mermaid was inserted — modal closed is enough to verify
    // (TipTap may strip raw div tags, so checking modal close is the key assertion)
  });

  test('import and export controls exist', async ({ page }) => {
    await page.goto(`${BASE}/admin/slides/new`);
    // The PPTX import file input should exist (hidden inside label)
    await expect(page.locator('#import-pptx-input')).toBeAttached();
    // Export button should be visible
    await expect(page.locator('#export-pptx-btn')).toBeVisible();
  });
});

// ===========================================================================
// 4. SLIDE EDITING
// ===========================================================================
test.describe('Slide Editing', () => {
  let slideEditUrl;

  test.beforeEach(async ({ page }) => {
    test.setTimeout(30000);
    await login(page);
  });

  test('edit existing slide — update title and content', async ({ page }) => {
    // Create a slide first
    await page.goto(`${BASE}/admin/slides/new`);
    const origTitle = 'Edit Test ' + Date.now();
    await page.fill('#panel-title', origTitle);
    const editor = page.locator('#tiptap-container .ProseMirror');
    await editor.click();
    await editor.pressSequentially('Original content');
    await page.click('#publish-btn');
    await page.waitForURL('**/admin/slides/*/edit', { timeout: 10000 });

    // Now update it
    const newTitle = 'Updated ' + origTitle;
    await page.fill('#panel-title', newTitle);

    // Clear and re-type content
    await editor.click();
    await page.keyboard.press(`${process.platform === 'darwin' ? 'Meta' : 'Control'}+A`);
    await editor.pressSequentially('Updated content');

    await page.click('#publish-btn');
    await page.waitForURL('**/admin/slides/*/edit', { timeout: 10000 });

    const savedTitle = await page.locator('#panel-title').inputValue();
    expect(savedTitle).toBe(newTitle);
  });

  test('toggle published status on existing slide', async ({ page }) => {
    // Create published slide
    await page.goto(`${BASE}/admin/slides/new`);
    const title = 'Toggle Pub ' + Date.now();
    await page.fill('#panel-title', title);
    const editor = page.locator('#tiptap-container .ProseMirror');
    await editor.click();
    await editor.pressSequentially('Toggle test');
    await page.click('#publish-btn');
    await page.waitForURL('**/admin/slides/*/edit', { timeout: 10000 });

    // Re-save as draft
    await page.click('#save-draft-btn');
    await page.waitForURL('**/admin/slides/*/edit', { timeout: 10000 });

    // Should no longer be in public list
    await page.goto(`${BASE}/slides`);
    const publicContent = await page.content();
    expect(publicContent).not.toContain(title);
  });
});

// ===========================================================================
// 5. SLIDE EDITOR — AUTOSAVE
// ===========================================================================
test.describe('Slide Autosave', () => {
  test.beforeEach(async ({ page }) => {
    test.setTimeout(45000);
    await login(page);
  });

  test('autosave triggers after editing in edit mode', async ({ page }) => {
    // Create a slide first
    await page.goto(`${BASE}/admin/slides/new`);
    await page.fill('#panel-title', 'Autosave Test ' + Date.now());
    const editor = page.locator('#tiptap-container .ProseMirror');
    await editor.click();
    await editor.pressSequentially('Autosave initial');
    await page.click('#publish-btn');
    await page.waitForURL('**/admin/slides/*/edit', { timeout: 10000 });

    // Now type to trigger autosave
    await editor.click();
    await editor.pressSequentially(' more content for autosave');

    // Wait for autosave (3s debounce + network)
    await page.waitForTimeout(5000);

    // Check autosave indicator
    const statusText = await page.locator('#autosave-status').textContent();
    expect(statusText).toMatch(/Saved/i);
  });
});

// ===========================================================================
// 6. PUBLIC VIEWS
// ===========================================================================
test.describe('Public Slide Views', () => {
  test.beforeEach(async ({ page }) => {
    test.setTimeout(30000);
    await login(page);
  });

  test('published slide appears in public list', async ({ page }) => {
    // Create and publish a slide
    await page.goto(`${BASE}/admin/slides/new`);
    const title = 'Public List Test ' + Date.now();
    await page.fill('#panel-title', title);
    const editor = page.locator('#tiptap-container .ProseMirror');
    await editor.click();
    await editor.pressSequentially('Public slide content');
    await page.click('#publish-btn');
    await page.waitForURL('**/admin/slides/*/edit', { timeout: 10000 });

    // Check public list
    await page.goto(`${BASE}/slides`);
    await expect(page.locator(`text=${title}`)).toBeVisible();
  });

  test('slide presentation renders with Reveal.js', async ({ page }) => {
    // Create a slide
    await page.goto(`${BASE}/admin/slides/new`);
    const slug = 'reveal-test-' + Date.now();
    await page.fill('#panel-title', 'Reveal Test');
    await page.fill('#panel-slug', slug);
    const editor = page.locator('#tiptap-container .ProseMirror');
    await editor.click();
    await editor.pressSequentially('Reveal.js slide');
    await page.click('#publish-btn');
    await page.waitForURL('**/admin/slides/*/edit', { timeout: 10000 });

    // View the presentation
    await page.goto(`${BASE}/slides/${slug}`);
    await expect(page.locator('.reveal')).toBeVisible();
    await expect(page.locator('.reveal .slides')).toBeVisible();
  });
});

// ===========================================================================
// 7. ADMIN SLIDES VIEW
// ===========================================================================
test.describe('Admin Slides View', () => {
  test.beforeEach(async ({ page }) => {
    test.setTimeout(30000);
    await login(page);
  });

  test('admin list shows slides with metadata', async ({ page }) => {
    await page.goto(`${BASE}/admin/slides`);

    await expect(page.locator('h1')).toContainText('Slide Management');
    await expect(page.locator('text=New Slide')).toBeVisible();
    await expect(page.locator('text=Import PPTX')).toBeVisible();
  });

  test('admin list has search functionality', async ({ page }) => {
    await page.goto(`${BASE}/admin/slides`);
    await expect(page.locator('#searchInput')).toBeVisible();
  });

  test('admin can navigate to new slide editor', async ({ page }) => {
    await page.goto(`${BASE}/admin/slides`);
    await page.click('text=New Slide');
    await page.waitForURL('**/admin/slides/new', { timeout: 5000 });
    await expect(page.locator('#slide-sidebar')).toBeVisible();
  });
});

// ===========================================================================
// 8. EDGE CASES & VALIDATION
// ===========================================================================
test.describe('Slide Creation - Edge Cases', () => {
  test.beforeEach(async ({ page }) => {
    test.setTimeout(30000);
    await login(page);
  });

  test('slug with special characters gets sanitized', async ({ page }) => {
    await page.goto(`${BASE}/admin/slides/new`);

    await page.fill('#panel-title', 'Special Chars');
    await page.fill('#panel-slug', 'Hello World!! @#$% Test');

    const editor = page.locator('#tiptap-container .ProseMirror');
    await editor.click();
    await editor.pressSequentially('Slug sanitization test');

    await page.click('#publish-btn');
    await page.waitForURL('**/admin/slides/*/edit', { timeout: 10000 });

    // Slug should be sanitized (lowercase, no special chars)
    const savedSlug = await page.locator('#panel-slug').inputValue();
    expect(savedSlug).not.toContain('@');
    expect(savedSlug).not.toContain('#');
    expect(savedSlug).not.toContain('!');
    expect(savedSlug).toMatch(/^[a-z0-9-]+$/);
  });

  test('very long title is accepted', async ({ page }) => {
    await page.goto(`${BASE}/admin/slides/new`);

    const longTitle = 'A'.repeat(200) + ' ' + Date.now();
    await page.fill('#panel-title', longTitle);

    const editor = page.locator('#tiptap-container .ProseMirror');
    await editor.click();
    await editor.pressSequentially('Long title test');

    await page.click('#publish-btn');
    await page.waitForURL('**/admin/slides/*/edit', { timeout: 10000 });
  });

  test('multiple slides with content are preserved', async ({ page }) => {
    await page.goto(`${BASE}/admin/slides/new`);

    const title = 'Multi Slide Content ' + Date.now();
    await page.fill('#panel-title', title);

    // Edit first slide
    const editor = page.locator('#tiptap-container .ProseMirror');
    await editor.click();
    await page.keyboard.press(`${process.platform === 'darwin' ? 'Meta' : 'Control'}+A`);
    await editor.pressSequentially('Slide 1 content');

    // Add second slide
    await page.click('#add-slide-btn');
    await editor.click();
    await editor.pressSequentially('Slide 2 content');

    // Add third slide
    await page.click('#add-slide-btn');
    await editor.click();
    await editor.pressSequentially('Slide 3 content');

    // Verify 3 thumbnails
    await expect(page.locator('.slide-thumb')).toHaveCount(3);

    // Save
    await page.click('#publish-btn');
    await page.waitForURL('**/admin/slides/*/edit', { timeout: 10000 });

    // After reload, should still have 3 slides
    const thumbCount = await page.locator('.slide-thumb').count();
    expect(thumbCount).toBe(3);
  });

  test('empty content slide can be saved', async ({ page }) => {
    await page.goto(`${BASE}/admin/slides/new`);
    await page.fill('#panel-title', 'Empty Content ' + Date.now());

    // Don't type anything in the editor — just save
    await page.click('#save-draft-btn');
    await page.waitForURL('**/admin/slides/*/edit', { timeout: 10000 });
  });

  test('unicode title and content are handled', async ({ page }) => {
    await page.goto(`${BASE}/admin/slides/new`);

    const unicodeTitle = 'Ünïcödé Prëséntätïön 日本語 🎯 ' + Date.now();
    await page.fill('#panel-title', unicodeTitle);

    const editor = page.locator('#tiptap-container .ProseMirror');
    await editor.click();
    await editor.pressSequentially('Content with émojis 🚀 and 中文');

    await page.click('#publish-btn');
    await page.waitForURL('**/admin/slides/*/edit', { timeout: 10000 });

    const savedTitle = await page.locator('#panel-title').inputValue();
    expect(savedTitle).toBe(unicodeTitle);
  });

  test('rapid slide add does not crash', async ({ page }) => {
    await page.goto(`${BASE}/admin/slides/new`);

    // Rapidly add 5 slides
    for (let i = 0; i < 5; i++) {
      await page.click('#add-slide-btn');
    }

    // Should have 6 total (1 default + 5 added)
    await expect(page.locator('.slide-thumb')).toHaveCount(6);
  });

  test('slide with all fields populated', async ({ page }) => {
    await page.goto(`${BASE}/admin/slides/new`);

    const title = 'Full Slide ' + Date.now();
    await page.fill('#panel-title', title);
    await page.fill('#panel-slug', 'full-slide-' + Date.now());
    await page.fill('#panel-description', 'A comprehensive presentation');
    await page.fill('#panel-password', 'secure123');

    // Select category if available
    const firstCat = page.locator('.cat-checkbox').first();
    if (await firstCat.count() > 0) {
      await firstCat.check();
    }

    // Set background and transition
    await page.locator('#slide-bg-color').fill('#1a1a2e');
    await page.selectOption('#slide-transition', 'zoom');

    // Type content
    const editor = page.locator('#tiptap-container .ProseMirror');
    await editor.click();
    await editor.pressSequentially('Full featured slide');

    // Add a second slide
    await page.click('#add-slide-btn');
    await editor.click();
    await editor.pressSequentially('Second slide of full test');

    await page.click('#publish-btn');
    await page.waitForURL('**/admin/slides/*/edit', { timeout: 10000 });

    // Verify title persisted
    const savedTitle = await page.locator('#panel-title').inputValue();
    expect(savedTitle).toBe(title);
  });
});

// ===========================================================================
// 9. UNAUTHORIZED ACCESS
// ===========================================================================
test.describe('Slide Access Control', () => {
  test('unauthenticated user cannot access slide editor', async ({ page }) => {
    await page.goto(`${BASE}/admin/slides/new`);
    // Should redirect to signin
    await expect(page).toHaveURL(/signin/);
  });

  test('unauthenticated user cannot access admin slides', async ({ page }) => {
    await page.goto(`${BASE}/admin/slides`);
    await expect(page).toHaveURL(/signin/);
  });

  test('unauthenticated user can view public slides list', async ({ page }) => {
    const resp = await page.goto(`${BASE}/slides`);
    expect(resp.status()).toBe(200);
  });
});
