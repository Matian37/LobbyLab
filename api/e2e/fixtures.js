/* eslint-disable no-empty-pattern */

import { test as base, expect } from '@playwright/test';
import postgres from 'postgres';
import { DATABASE_URL } from './helpers.js';
import { resetDb, removeClientFile } from './helpers.js';

export const test = base.extend({
    helperSql: async ({}, use) => {
        const sql = postgres(DATABASE_URL, { onnotice: () => {} });
        await use(sql);
        await sql.end();
    },
    resetSchema: [
        async ({ helperSql }, use) => {
            await resetDb(helperSql);
            await use();
        },
        { auto: true },
    ],
    resetClientFile: [
        async ({}, use) => {
            removeClientFile();
            await use();
        },
        { auto: true },
    ],
});
export { expect };
