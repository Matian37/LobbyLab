import { test, expect } from './fixtures.js';
import { registerViaUi, createMatchResult } from './helpers.js';

async function getTableMatrix(table) {
    return table.evaluate((element) =>
        [...element.querySelectorAll('tr')].map((row) =>
            [...row.querySelectorAll('td, th')].map(
                (cell) => cell.textContent?.trim() ?? ''
            )
        )
    );
}

test.describe('results', () => {
    test('shows an empty results table for a new user', async ({ page }) => {
        await registerViaUi(page, 'login');

        const table = page.getByRole('table');
        await expect
            .poll(() => getTableMatrix(table))
            .toEqual([['ID', 'STATUS', 'PLAYERS', 'WINNER']]);
    });

    test('shows matches in the results table', async ({ page, helperSql }) => {
        await registerViaUi(page, 'login');

        const finishedMatchId = await createMatchResult(helperSql, ['login'], {
            players: ['login', 'other'],
            winner: 'login',
        });
        const canceledMatchId = await createMatchResult(helperSql, ['login'], {
            canceled: true,
        });
        const activeMatchId = await createMatchResult(helperSql, ['login']);

        await page.reload();

        const table = page.getByRole('table');
        await expect
            .poll(() => getTableMatrix(table))
            .toEqual([
                ['ID', 'STATUS', 'PLAYERS', 'WINNER'],
                [String(activeMatchId), 'ACTIVE', '', ''],
                [String(canceledMatchId), 'CANCELED', '', ''],
                [String(finishedMatchId), 'FINISHED', 'login, other', 'login'],
            ]);
    });

    test('shows only the matches the user participated in', async ({
        browser,
        helperSql,
    }) => {
        const context1 = await browser.newContext();
        const context2 = await browser.newContext();
        const page1 = await context1.newPage();
        const page2 = await context2.newPage();

        try {
            await registerViaUi(page1, 'user1');
            await registerViaUi(page2, 'user2');

            const onlyUser1Id = await createMatchResult(helperSql, ['user1']);
            const onlyUser2Id = await createMatchResult(helperSql, ['user2'], {
                canceled: true,
            });
            const bothId = await createMatchResult(
                helperSql,
                ['user1', 'user2'],
                {
                    players: ['user1', 'user2'],
                    winner: 'user1',
                }
            );

            await page1.reload();
            await page2.reload();

            await expect
                .poll(() => getTableMatrix(page1.getByRole('table')))
                .toEqual([
                    ['ID', 'STATUS', 'PLAYERS', 'WINNER'],
                    [String(bothId), 'FINISHED', 'user1, user2', 'user1'],
                    [String(onlyUser1Id), 'ACTIVE', '', ''],
                ]);

            await expect
                .poll(() => getTableMatrix(page2.getByRole('table')))
                .toEqual([
                    ['ID', 'STATUS', 'PLAYERS', 'WINNER'],
                    [String(bothId), 'FINISHED', 'user1, user2', 'user1'],
                    [String(onlyUser2Id), 'CANCELED', '', ''],
                ]);
        } finally {
            await context1.close();
            await context2.close();
        }
    });
});
