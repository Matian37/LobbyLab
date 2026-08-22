import { test, expect } from './fixtures.js';
import { BASE_URL, registerViaUi, assignMatch } from './helpers.js';

function expectedGameUrl(token, { host = 'game-host', port = '7777' } = {}) {
    return `http://game/${encodeURIComponent(host)}/${encodeURIComponent(port)}/${encodeURIComponent(token)}`;
}

async function getMatchAuthToken(sql, login) {
    const rows = await sql`
        SELECT match_auth_token FROM users WHERE login = ${login}
    `;
    return rows[0].match_auth_token;
}

async function isWaiting(sql, login) {
    const rows = await sql`
        SELECT match_id IS NULL AND queued_until > NOW() AS waiting
        FROM users
        WHERE login = ${login}
    `;
    return rows[0]?.waiting === true;
}

test.describe('matchmaking', () => {
    test('shows join button when user already in match', async ({
        page,
        helperSql,
    }) => {
        await registerViaUi(page, 'login');
        await assignMatch(helperSql, ['login'], {
            host: 'game-host',
            port: '7777',
        });

        await page.route('http://game/**', (route) =>
            route.fulfill({
                status: 200,
                contentType: 'text/html',
                body: '<html></html>',
            })
        );

        await page.goto(`${BASE_URL}/`);

        await expect(page.getByTestId('play')).toHaveText('Join');

        await page.getByTestId('play').click();

        const token = await getMatchAuthToken(helperSql, 'login');
        await page.waitForURL(expectedGameUrl(token));
        expect(page.url()).toBe(expectedGameUrl(token));
    });

    test('on connection auto-reloads and joins when user already in match', async ({
        page,
        helperSql,
    }) => {
        await registerViaUi(page, 'login');
        await assignMatch(helperSql, ['login'], {
            host: 'game-host',
            port: '7777',
        });

        await page.route('http://game/**', (route) =>
            route.fulfill({
                status: 200,
                contentType: 'text/html',
                body: '<html></html>',
            })
        );

        await expect(page.getByTestId('play')).toHaveText('Play');
        await page.getByTestId('play').click();

        await expect(page.getByTestId('play')).toHaveText('Join');
        await page.getByTestId('play').click();

        const token = await getMatchAuthToken(helperSql, 'login');
        await page.waitForURL(expectedGameUrl(token));
        expect(page.url()).toBe(expectedGameUrl(token));
    });

    test('launches the game when a match is assigned while waiting', async ({
        page,
        helperSql,
    }) => {
        await registerViaUi(page, 'login');

        await page.route('http://game/**', (route) =>
            route.fulfill({
                status: 200,
                contentType: 'text/html',
                body: '<html></html>',
            })
        );

        await page.getByTestId('play').click();
        await expect(page.getByTestId('play')).toHaveText(/Cancel/);
        await expect
            .poll(() => isWaiting(helperSql, 'login'), { timeout: 10_000 })
            .toBe(true);

        await assignMatch(helperSql, ['login']);

        const token = await getMatchAuthToken(helperSql, 'login');
        await page.waitForURL(expectedGameUrl(token));
        expect(page.url()).toBe(expectedGameUrl(token));
    });

    test('shows an error when the session cookie becomes invalid', async ({
        page,
    }) => {
        await registerViaUi(page, 'login');
        await expect(page.getByTestId('play')).toHaveText('Play');

        await page.context().clearCookies();

        await page.getByTestId('play').click();

        await expect(page.getByTestId('error')).toHaveText(
            'Connection issue, try again later'
        );
        await expect(page.getByTestId('play')).toHaveText('Play');
    });

    test('shows an error when the connection is replaced by a newer one', async ({
        page,
        helperSql,
    }) => {
        await registerViaUi(page, 'login');
        const page2 = await page.context().newPage();

        try {
            await page.getByTestId('play').click();
            await expect(page.getByTestId('play')).toHaveText(/Cancel/);
            await expect
                .poll(() => isWaiting(helperSql, 'login'), { timeout: 10_000 })
                .toBe(true);

            await page2.goto(`${BASE_URL}/`);
            await page2.getByTestId('play').click();
            await expect(page2.getByTestId('play')).toHaveText(/Cancel/);

            await expect(page.getByTestId('error')).toHaveText(
                'Connection issue, try again later'
            );
            await expect(page.getByTestId('play')).toHaveText('Play');
        } finally {
            await page2.close();
        }
    });

    test('removes the queue entry when matchmaking is cancelled', async ({
        page,
        helperSql,
    }) => {
        await registerViaUi(page, 'login');

        await page.getByTestId('play').click();
        await expect(page.getByTestId('play')).toHaveText(/Cancel/);
        await expect
            .poll(() => isWaiting(helperSql, 'login'), { timeout: 10_000 })
            .toBe(true);

        await page.getByTestId('play').click();

        await expect
            .poll(async () => !(await isWaiting(helperSql, 'login')), {
                timeout: 10_000,
            })
            .toBe(true);
    });

    test('queues two users and launches the game when a match is assigned', async ({
        browser,
        helperSql,
    }) => {
        const context1 = await browser.newContext();
        const context2 = await browser.newContext();
        await context1.route('http://game/**', (route) =>
            route.fulfill({
                status: 200,
                contentType: 'text/html',
                body: '<html></html>',
            })
        );
        await context2.route('http://game/**', (route) =>
            route.fulfill({
                status: 200,
                contentType: 'text/html',
                body: '<html></html>',
            })
        );
        const page1 = await context1.newPage();
        const page2 = await context2.newPage();

        try {
            await registerViaUi(page1, 'user1');
            await registerViaUi(page2, 'user2');

            await page1.getByTestId('play').click();
            await page2.getByTestId('play').click();

            await expect(page1.getByTestId('play')).toHaveText(/Cancel/);
            await expect(page2.getByTestId('play')).toHaveText(/Cancel/);

            await expect
                .poll(() => isWaiting(helperSql, 'user1'), { timeout: 10_000 })
                .toBe(true);
            await expect
                .poll(() => isWaiting(helperSql, 'user2'), { timeout: 10_000 })
                .toBe(true);

            await assignMatch(helperSql, ['user1', 'user2'], {
                host: 'game-host',
                port: '7777',
            });

            const token1 = await getMatchAuthToken(helperSql, 'user1');
            const token2 = await getMatchAuthToken(helperSql, 'user2');

            await Promise.all([
                page1.waitForURL(expectedGameUrl(token1)),
                page2.waitForURL(expectedGameUrl(token2)),
            ]);

            expect(page1.url()).toBe(expectedGameUrl(token1));
            expect(page2.url()).toBe(expectedGameUrl(token2));
        } finally {
            await context1.close();
            await context2.close();
        }
    });
});
