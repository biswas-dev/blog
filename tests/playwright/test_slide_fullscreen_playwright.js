#!/usr/bin/env node

const { chromium } = require('playwright');

async function testSlideFullscreenPadding() {
    let browser;
    let context;

    try {
        console.log('🚀 Launching Chromium...');
        browser = await chromium.launch({ headless: true });
        context = await browser.newContext({
            viewport: { width: 1440, height: 900 }
        });
        const page = await context.newPage();

        // Fast-fail defaults
        page.setDefaultTimeout(7000);
        page.setDefaultNavigationTimeout(7000);

        const slideUrl = 'https://anshumanbiswas.com/slides/vibe-coding-to-production-mastering-cursor-ai';

        // Stub the Fullscreen API so that it can be triggered in headless mode
        await page.addInitScript(() => {
            const fullscreenState = { element: null };
            const changeEvents = ['fullscreenchange', 'webkitfullscreenchange', 'mozfullscreenchange', 'MSFullscreenChange'];

            const dispatchFullscreenEvents = () => {
                changeEvents.forEach(eventName => {
                    document.dispatchEvent(new Event(eventName));
                });
            };

            const requestFullscreen = function() {
                fullscreenState.element = this;
                dispatchFullscreenEvents();
                return Promise.resolve();
            };

            const exitFullscreen = () => {
                fullscreenState.element = null;
                dispatchFullscreenEvents();
                return Promise.resolve();
            };

            const defineFullscreenElementProp = (prop) => {
                try {
                    Object.defineProperty(document, prop, {
                        configurable: true,
                        get() {
                            return fullscreenState.element;
                        }
                    });
                } catch (err) {
                    // Ignore environments that do not allow redefining the property
                }
            };

            ['fullscreenElement', 'webkitFullscreenElement', 'mozFullScreenElement', 'msFullscreenElement'].forEach(defineFullscreenElementProp);
            ['exitFullscreen', 'webkitExitFullscreen', 'mozCancelFullScreen', 'msExitFullscreen'].forEach(prop => {
                try {
                    document[prop] = exitFullscreen;
                } catch (err) {
                    // Ignore environments that do not allow overriding the property
                }
            });

            ['requestFullscreen', 'webkitRequestFullscreen', 'mozRequestFullScreen', 'msRequestFullscreen'].forEach(prop => {
                try {
                    Object.defineProperty(Element.prototype, prop, {
                        configurable: true,
                        writable: true,
                        value: requestFullscreen
                    });
                } catch (err) {
                    // Ignore if we cannot define the property
                }
            });
        });

        console.log(`🌐 Navigating to ${slideUrl} ...`);
        await page.goto(slideUrl, { waitUntil: 'domcontentloaded' });
        await page.waitForLoadState('networkidle', { timeout: 5000 }).catch(() => {});

        const firstSectionLocator = page.locator('.reveal .slides section').first();
        await firstSectionLocator.waitFor({ timeout: 5000 });

        const initialPaddingTop = await firstSectionLocator.evaluate(node => parseFloat(getComputedStyle(node).paddingTop));
        console.log(`   • Initial padding-top: ${initialPaddingTop.toFixed(2)}px`);

        console.log('🖥️ Triggering fullscreen mode...');
        await page.click('#fullscreen-btn');
        await page.waitForFunction(() => document.body.classList.contains('is-fullscreen'), { timeout: 3000 });

        const fullscreenPaddingTop = await firstSectionLocator.evaluate(node => parseFloat(getComputedStyle(node).paddingTop));
        console.log(`   • Fullscreen padding-top: ${fullscreenPaddingTop.toFixed(2)}px`);

        if (fullscreenPaddingTop < 130) {
            throw new Error(`Expected fullscreen padding-top >= 130px, received ${fullscreenPaddingTop}px`);
        }

        const headingLocator = page.locator('.reveal .slides section h1').first();
        await headingLocator.waitFor({ timeout: 3000 });
        const headingBox = await headingLocator.boundingBox();

        if (!headingBox) {
            throw new Error('Unable to determine heading bounding box');
        }

        console.log(`   • Heading Y position in fullscreen: ${headingBox.y.toFixed(2)}px`);

        if (headingBox.y < 12) {
            throw new Error(`Fullscreen heading is too close to the top edge (y=${headingBox.y})`);
        }

        console.log('↩️ Exiting fullscreen mode...');
        await page.click('#fullscreen-btn');
        await page.waitForFunction(() => !document.body.classList.contains('is-fullscreen'), { timeout: 3000 });

        console.log('✅ Slide fullscreen spacing looks good!');
    } catch (error) {
        console.error('❌ Slide fullscreen test failed:', error.message);
        console.error(error);
        process.exit(1);
    } finally {
        if (context) {
            await context.close();
        }
        if (browser) {
            await browser.close();
        }
    }
}

testSlideFullscreenPadding();
