import { test, expect } from './fixtures.js';
import {
    BASE_URL,
    registerViaUi,
    addClientFile,
    removeClientFile,
    CLIENT_FILE,
    CLIENT_CONTENT,
} from './helpers.js';

test.afterEach(removeClientFile);

test('hides the download button when logged out', async ({ page }) => {
    await page.goto(BASE_URL + '/');

    await expect(page.getByTestId('download-client')).not.toBeVisible();
});

test('downloads the game client archive when logged in', async ({ page }) => {
    await registerViaUi(page, 'login');
    await addClientFile();

    const downloadPromise = page.waitForEvent('download');
    await page.getByTestId('download-client').click();
    const download = await downloadPromise;

    expect(download.suggestedFilename()).toBe(CLIENT_FILE);

    const stream = await download.createReadStream();
    const chunks = [];
    for await (const chunk of stream) chunks.push(chunk);
    expect(Buffer.concat(chunks).toString()).toBe(CLIENT_CONTENT);
});
