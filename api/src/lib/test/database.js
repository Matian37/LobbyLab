import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { PostgreSqlContainer } from '@testcontainers/postgresql';
import postgres from 'postgres';

const __dirname = path.dirname(fileURLToPath(import.meta.url));

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

export async function resetSchema(helperSql) {
    await helperSql.unsafe('DROP SCHEMA public CASCADE; CREATE SCHEMA public;');
    await helperSql.unsafe(process.env.DATABASE_INIT_SQL);
}

export async function teardownDatabase({ container, helperSql, sqlPool }) {
    if (helperSql) await helperSql.end();
    if (sqlPool) await sqlPool.end();

    if (container) {
        console.log('[db] stopping container...');
        await container.stop();
        console.log('[db] container stopped');
    }
}
