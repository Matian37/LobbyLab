import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { PostgreSqlContainer } from '@testcontainers/postgresql';
import postgres from 'postgres';

const __dirname = path.dirname(fileURLToPath(import.meta.url));

/**
 * Starts a disposable PostgreSQL test container, initializes the schema from
 * `init.sql`, and returns handles for the test to manage it. The resulting
 * `DATABASE_URL` is also exported to `process.env` so the app's shared `sql`
 * pool connects to it.
 *
 * @param {{ port?: number }} [options] If a `port` is given, the container's
 *     PostgreSQL port is mapped to that host port.
 * @returns {Promise<{
 *     container: import('testcontainers').StartedTestContainer,
 *     databaseUrl: string,
 *     helperSql: import('postgres').Sql
 * }>} Handles for teardown and direct queries against the test database.
 */
export async function setupDatabase({ port } = {}) {
    console.log('[db] starting container...');

    let setupContainer = await new PostgreSqlContainer('postgres:18.4-alpine')
        .withUsername('postgres')
        .withPassword('123')
        .withDatabase('postgres');
    if (port) {
        setupContainer = setupContainer.withExposedPorts({
            container: 5432,
            host: port,
        });
    }

    const container = await setupContainer.start();

    const databaseUrl = container.getConnectionUri();
    process.env.DATABASE_URL = databaseUrl;
    console.log('[db] container started; initializing schema...');

    const initSqlPath = path.resolve(__dirname, '../../../../init.sql');
    const initSql = fs.readFileSync(initSqlPath, 'utf8');
    process.env.DATABASE_INIT_SQL = initSql;

    const pg = postgres(databaseUrl, { onnotice: () => {} });
    await pg.unsafe(initSql);
    await pg.end();

    console.log('[db] schema initialized; creating helper connection...');

    const helperSql = postgres(databaseUrl, { onnotice: () => {} });

    console.log('[db] helper connection created; database ready');

    return { container, databaseUrl, helperSql };
}

/**
 * Drops and recreates the `public` schema, then re-runs `init.sql` so each test
 * starts from a clean, freshly initialized state.
 *
 * @param {import('postgres').Sql} helperSql A connection to the test database.
 * @returns {Promise<void>}
 */
export async function resetSchema(helperSql) {
    await helperSql.unsafe('DROP SCHEMA public CASCADE; CREATE SCHEMA public;');
    await helperSql.unsafe(process.env.DATABASE_INIT_SQL);
}

/**
 * Stops the test container and closes any open connections.
 *
 * @param {{
 *     container?: import('testcontainers').StartedTestContainer,
 *     helperSql?: import('postgres').Sql,
 *     sqlPool?: import('postgres').Sql
 * }} handles Handles returned by {@link setupDatabase}.
 * @returns {Promise<void>}
 */
export async function teardownDatabase({ container, helperSql, sqlPool }) {
    if (helperSql) await helperSql.end();
    if (sqlPool) await sqlPool.end();

    if (container) {
        console.log('[db] stopping container...');
        await container.stop();
        console.log('[db] container stopped');
    }
}
