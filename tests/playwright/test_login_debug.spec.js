const { test, expect } = require('@playwright/test');
const http = require('http');

function postLogin() {
  return new Promise((resolve, reject) => {
    const postData = 'email=anchoo2kewl@gmail.com&password=123456qwertyu';
    const req = http.request({
      hostname: 'localhost',
      port: 22222,
      path: '/signin',
      method: 'POST',
      headers: {
        'Content-Type': 'application/x-www-form-urlencoded',
        'Content-Length': Buffer.byteLength(postData),
      },
    }, (res) => {
      const cookies = res.headers['set-cookie'] || [];
      resolve({ status: res.statusCode, cookies, location: res.headers.location });
      res.resume(); // drain the response
    });
    req.on('error', reject);
    req.write(postData);
    req.end();
  });
}

test('login via Node http and inject cookies', async ({ page, context }) => {
  test.setTimeout(15000);

  // Do login via raw HTTP (no redirect following)
  const result = await postLogin();
  console.log('Login status:', result.status, 'Location:', result.location);
  console.log('Cookies:', result.cookies);

  expect(result.status).toBe(302);

  // Parse and inject cookies into the browser context
  for (const cookieStr of result.cookies) {
    const parts = cookieStr.split(';')[0].split('=');
    const name = parts[0].trim();
    const value = parts.slice(1).join('=').trim();
    await context.addCookies([{
      name,
      value,
      domain: 'localhost',
      path: '/',
    }]);
  }

  // Now navigate to admin slides
  await page.goto('http://localhost:22222/admin/slides', { waitUntil: 'domcontentloaded' });
  console.log('URL:', page.url());
  expect(page.url()).toContain('admin/slides');
});
