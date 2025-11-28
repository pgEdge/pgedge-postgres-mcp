/*-------------------------------------------------------------------------
 *
 * Screenshot automation script for pgEdge NLA documentation
 *
 * Usage:
 *   node scripts/take-screenshots.js [options]
 *
 * Options:
 *   --url=<url>        Base URL (default: http://localhost:5173)
 *   --output=<dir>     Output directory (default: ../docs/img/screenshots)
 *   --user=<username>  Username for login (required for most screenshots)
 *   --pass=<password>  Password for login (required for most screenshots)
 *   --viewport=<WxH>   Viewport size (default: 1280x800)
 *   --query=<text>     Query to execute (default: "List all tables...")
 *   --help             Show this help message
 *
 * Portions copyright (c) 2025, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

import puppeteer from 'puppeteer';
import path from 'path';
import fs from 'fs';
import { fileURLToPath } from 'url';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

// Helper function to wait (replacement for deprecated waitForTimeout)
const delay = (ms) => new Promise(resolve => setTimeout(resolve, ms));

// Parse command line arguments
function parseArgs() {
    const args = {
        url: 'http://localhost:5173',
        output: path.join(__dirname, '../../docs/img/screenshots'),
        user: null,
        pass: null,
        viewport: { width: 1280, height: 800 },
        query: 'List all tables in the database with their row counts',
        help: false,
    };

    for (const arg of process.argv.slice(2)) {
        if (arg === '--help' || arg === '-h') {
            args.help = true;
        } else if (arg.startsWith('--url=')) {
            args.url = arg.split('=')[1];
        } else if (arg.startsWith('--output=')) {
            args.output = arg.split('=')[1];
        } else if (arg.startsWith('--user=')) {
            args.user = arg.split('=')[1];
        } else if (arg.startsWith('--pass=')) {
            args.pass = arg.split('=')[1];
        } else if (arg.startsWith('--viewport=')) {
            const [w, h] = arg.split('=')[1].split('x').map(Number);
            args.viewport = { width: w, height: h };
        } else if (arg.startsWith('--query=')) {
            args.query = arg.split('=').slice(1).join('=');
        }
    }

    return args;
}

function showHelp() {
    console.log(`
Screenshot automation script for pgEdge NLA documentation

Usage:
  node scripts/take-screenshots.js [options]

Options:
  --url=<url>        Base URL (default: http://localhost:5173)
  --output=<dir>     Output directory (default: ../docs/img/screenshots)
  --user=<username>  Username for login (required for most screenshots)
  --pass=<password>  Password for login (required for most screenshots)
  --viewport=<WxH>   Viewport size (default: 1280x800)
  --query=<text>     Query to execute (default: "List all tables...")
  --help             Show this help message

Examples:
  # Take screenshot of login page only
  node scripts/take-screenshots.js

  # Take all screenshots including logged-in views
  node scripts/take-screenshots.js --user=admin --pass=secret

  # Custom viewport for mobile docs
  node scripts/take-screenshots.js --viewport=375x667 --user=admin --pass=secret
`);
}

let screenshotCounter = 0;

async function takeScreenshot(page, outputDir, name, description) {
    screenshotCounter++;
    const paddedNum = String(screenshotCounter).padStart(2, '0');
    const filepath = path.join(outputDir, `${paddedNum}-${name}.png`);
    console.log(`  [${paddedNum}] ${description}`);
    await page.screenshot({ path: filepath, fullPage: false });
    return filepath;
}

async function waitForLLMResponse(page, timeout = 120000) {
    console.log('  Waiting for LLM response...');
    const startTime = Date.now();

    // First wait for the textarea to become disabled (request started)
    let requestStarted = false;
    while (Date.now() - startTime < 5000) {
        const isDisabled = await page.evaluate(() => {
            const textArea = document.querySelector('textarea');
            return textArea?.disabled || false;
        });
        if (isDisabled) {
            requestStarted = true;
            console.log('  Request started...');
            break;
        }
        await delay(100);
    }

    if (!requestStarted) {
        console.log('  Warning: Request may not have started');
    }

    // Now wait for response (textarea becomes enabled again)
    while (Date.now() - startTime < timeout) {
        const isThinking = await page.evaluate(() => {
            const textArea = document.querySelector('textarea');
            return textArea?.disabled || false;
        });

        if (!isThinking && requestStarted) {
            // Double-check by waiting a bit
            await delay(1000);
            const stillThinking = await page.evaluate(() => {
                const textArea = document.querySelector('textarea');
                return textArea?.disabled || false;
            });
            if (!stillThinking) {
                console.log('  LLM response received!');
                return true;
            }
        }

        await delay(500);
    }

    console.log('  Warning: Timeout waiting for LLM response');
    return false;
}

// Open a MUI Select dropdown using Puppeteer click
async function openMuiSelect(page, selectId) {
    // MUI Select uses role="combobox" - find and click it
    const selector = `#${selectId}`;

    try {
        // Wait for element to exist
        await page.waitForSelector(selector, { timeout: 2000 });

        // Click the select element directly using Puppeteer
        await page.click(selector);
        await delay(400);

        // Check if menu appeared
        const menuExists = await page.$('.MuiMenu-paper, .MuiPopover-paper');
        return menuExists !== null;
    } catch (e) {
        console.log(`  Could not open select ${selectId}: ${e.message}`);
        return false;
    }
}

async function closeMuiDropdown(page) {
    await page.keyboard.press('Escape');
    await delay(300);
}

// Toggle dark/light mode
async function setDarkMode(page, enableDark) {
    const isDarkNow = await page.evaluate(() => {
        const appBar = document.querySelector('.MuiAppBar-root');
        if (appBar) {
            const bg = window.getComputedStyle(appBar).backgroundColor;
            const match = bg.match(/rgb\((\d+),\s*(\d+),\s*(\d+)\)/);
            if (match) {
                const brightness = (parseInt(match[1]) + parseInt(match[2]) + parseInt(match[3])) / 3;
                return brightness < 50;
            }
        }
        return false;
    });

    if (enableDark !== isDarkNow) {
        const clicked = await page.evaluate(() => {
            const btn = document.querySelector('[aria-label="toggle theme"]');
            if (btn) {
                btn.click();
                return true;
            }
            return false;
        });

        if (clicked) {
            await delay(500);
            return true;
        }
    }
    return false;
}

async function takeScreenshots() {
    const args = parseArgs();

    if (args.help) {
        showHelp();
        process.exit(0);
    }

    // Ensure output directory exists
    if (!fs.existsSync(args.output)) {
        fs.mkdirSync(args.output, { recursive: true });
    }

    // Clear existing screenshots
    const existingFiles = fs.readdirSync(args.output).filter(f => f.endsWith('.png'));
    if (existingFiles.length > 0) {
        console.log(`Clearing ${existingFiles.length} existing screenshots...`);
        existingFiles.forEach(f => fs.unlinkSync(path.join(args.output, f)));
    }

    console.log('='.repeat(60));
    console.log('pgEdge NLA Screenshot Automation');
    console.log('='.repeat(60));
    console.log(`URL:      ${args.url}`);
    console.log(`Output:   ${args.output}`);
    console.log(`Viewport: ${args.viewport.width}x${args.viewport.height}`);
    console.log('='.repeat(60));

    const browser = await puppeteer.launch({
        headless: true,
        args: ['--no-sandbox', '--disable-setuid-sandbox'],
    });

    try {
        const page = await browser.newPage();
        await page.setViewport(args.viewport);

        // =====================================================================
        // SECTION 1: Login Page
        // =====================================================================
        console.log('\n--- LOGIN ---');

        await page.goto(args.url, { waitUntil: 'networkidle0', timeout: 30000 });
        await delay(2000);

        await takeScreenshot(page, args.output, 'login-page',
            'Login page with animated background');

        if (!args.user || !args.pass) {
            console.log('\nNote: Credentials not provided. Only login screenshot taken.');
            console.log('Use --user=<username> --pass=<password> for full screenshot set.');
        } else {
            // Perform login
            console.log('\nLogging in...');
            await page.waitForSelector('input[name="username"]');
            await page.type('input[name="username"]', args.user);
            await page.type('input[name="password"]', args.pass);
            await page.click('button[type="submit"]');

            try {
                await page.waitForNavigation({ waitUntil: 'networkidle0', timeout: 10000 });
            } catch (e) {
                await delay(3000);
            }

            const textarea = await page.$('textarea');
            if (!textarea) {
                console.log('  Login may have failed.');
                await takeScreenshot(page, args.output, 'login-failed',
                    'Login result (may have failed)');
                throw new Error('Login failed - textarea not found');
            }

            await delay(2000);

            // =====================================================================
            // SECTION 2: Main Interface (Empty, Light Mode)
            // =====================================================================
            console.log('\n--- MAIN INTERFACE ---');

            await takeScreenshot(page, args.output, 'main-interface-empty',
                'Main chat interface (empty, light mode)');

            // =====================================================================
            // SECTION 3: Provider Dropdown
            // =====================================================================
            console.log('\n--- PROVIDER SELECTION ---');

            if (await openMuiSelect(page, 'provider-select')) {
                await takeScreenshot(page, args.output, 'provider-dropdown',
                    'Provider selection dropdown expanded');
                await closeMuiDropdown(page);
            } else {
                console.log('  Provider dropdown did not open');
            }

            // =====================================================================
            // SECTION 4: Model Dropdown
            // =====================================================================
            console.log('\n--- MODEL SELECTION ---');

            if (await openMuiSelect(page, 'model-select')) {
                await takeScreenshot(page, args.output, 'model-dropdown',
                    'Model selection dropdown expanded');
                await closeMuiDropdown(page);
            } else {
                console.log('  Model dropdown did not open');
            }

            // =====================================================================
            // SECTION 5: Execute Query and Show Response
            // =====================================================================
            console.log('\n--- QUERY EXECUTION ---');
            console.log(`  Query: "${args.query}"`);

            const textareaRefresh = await page.$('textarea');
            await textareaRefresh.type(args.query);
            await delay(500);

            await takeScreenshot(page, args.output, 'query-typed',
                'Query typed in message input');

            // Click send button
            const sendClicked = await page.evaluate(() => {
                const icons = document.querySelectorAll('svg[data-testid="SendIcon"]');
                for (const icon of icons) {
                    const btn = icon.closest('button');
                    if (btn && !btn.disabled) {
                        btn.click();
                        return true;
                    }
                }
                return false;
            });

            if (sendClicked) {
                await waitForLLMResponse(page, 120000);
                await delay(1500);

                await takeScreenshot(page, args.output, 'query-response-light',
                    'LLM response displayed (light mode)');
            } else {
                console.log('  Send button not found or disabled');
            }

            // =====================================================================
            // SECTION 6: Dark Mode with Response
            // =====================================================================
            console.log('\n--- DARK MODE ---');

            await setDarkMode(page, true);
            await takeScreenshot(page, args.output, 'query-response-dark',
                'LLM response displayed (dark mode)');

            // Switch back to light mode
            await setDarkMode(page, false);

            // =====================================================================
            // SECTION 7: Help Panel
            // =====================================================================
            console.log('\n--- HELP PANEL ---');

            const helpOpened = await page.evaluate(() => {
                const btn = document.querySelector('[aria-label="open help"]');
                if (btn) {
                    btn.click();
                    return true;
                }
                return false;
            });

            if (helpOpened) {
                await delay(500);
                await takeScreenshot(page, args.output, 'help-panel',
                    'Help panel drawer open');

                const closeClicked = await page.evaluate(() => {
                    const btn = document.querySelector('[aria-label="close help"]');
                    if (btn) {
                        btn.click();
                        return true;
                    }
                    return false;
                });

                if (!closeClicked) {
                    await page.keyboard.press('Escape');
                }
                await delay(300);
            } else {
                console.log('  Help button not found');
            }

            // =====================================================================
            // SECTION 8: Preferences Popover
            // =====================================================================
            console.log('\n--- PREFERENCES ---');

            const prefsOpened = await page.evaluate(() => {
                const icons = document.querySelectorAll('svg[data-testid="SettingsIcon"]');
                for (const icon of icons) {
                    const btn = icon.closest('button');
                    if (btn) {
                        btn.click();
                        return true;
                    }
                }
                return false;
            });

            if (prefsOpened) {
                await delay(400);
                await takeScreenshot(page, args.output, 'preferences-popover',
                    'Preferences popover with toggle switches');
                await closeMuiDropdown(page);
            } else {
                console.log('  Settings button not found');
            }

            // =====================================================================
            // SECTION 9: Prompt Workflow
            // =====================================================================
            console.log('\n--- PROMPT WORKFLOW ---');

            const promptOpened = await page.evaluate(() => {
                const icons = document.querySelectorAll('svg[data-testid="PsychologyIcon"]');
                for (const icon of icons) {
                    const btn = icon.closest('button');
                    if (btn) {
                        btn.click();
                        return true;
                    }
                }
                return false;
            });

            if (promptOpened) {
                await delay(400);

                const hasPopover = await page.$('.MuiPopover-paper');
                if (hasPopover) {
                    await takeScreenshot(page, args.output, 'prompt-popover-initial',
                        'Prompt popover before selecting a prompt');

                    // Open prompt dropdown
                    if (await openMuiSelect(page, 'prompt-popover-select')) {
                        await takeScreenshot(page, args.output, 'prompt-list-expanded',
                            'Prompt dropdown showing available prompts');

                        // Select first real prompt (skip placeholder)
                        const menuItems = await page.$$('.MuiMenuItem-root');
                        if (menuItems.length > 1) {
                            await menuItems[1].click();
                            await delay(500);

                            await takeScreenshot(page, args.output, 'prompt-selected',
                                'Prompt selected with description and arguments');

                            // Fill argument fields if present
                            const argInputs = await page.$$('.MuiPopover-paper .MuiTextField-root input');
                            if (argInputs.length > 0) {
                                for (const input of argInputs) {
                                    await input.type('example_value');
                                    await delay(150);
                                }
                                await takeScreenshot(page, args.output, 'prompt-args-filled',
                                    'Prompt with argument values entered');
                            }
                        }
                    } else {
                        console.log('  Prompt dropdown did not open');
                    }

                    await closeMuiDropdown(page);
                }
            } else {
                console.log('  Prompt button not found');
            }

            // =====================================================================
            // SECTION 10: User Menu
            // =====================================================================
            console.log('\n--- USER MENU ---');

            const userMenuOpened = await page.evaluate(() => {
                const btn = document.querySelector('[aria-label="user menu"]');
                if (btn) {
                    btn.click();
                    return true;
                }
                return false;
            });

            if (userMenuOpened) {
                await delay(300);
                await takeScreenshot(page, args.output, 'user-menu',
                    'User menu showing logout option');
                await closeMuiDropdown(page);
            } else {
                console.log('  User menu button not found');
            }
        }

        // =====================================================================
        // Summary
        // =====================================================================
        console.log('\n' + '='.repeat(60));
        console.log('Screenshot capture complete!');
        console.log('='.repeat(60));
        console.log(`Output directory: ${args.output}`);
        console.log(`\nFiles created (${screenshotCounter} total):`);

        const files = fs.readdirSync(args.output)
            .filter(f => f.endsWith('.png'))
            .sort();
        files.forEach(f => console.log(`  - ${f}`));

    } catch (error) {
        console.error('\nError taking screenshots:', error.message);
        process.exit(1);
    } finally {
        await browser.close();
    }
}

// Run the script
takeScreenshots();
