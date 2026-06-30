import { PostgreSqlContainer } from "@testcontainers/postgresql";
import postgres from "postgres";
import fs from "fs";
import path from "path";

export async function setup() {
  console.log(
    "Starting PostgreSQL container via @testcontainers/postgresql...",
  );

  const container = await new PostgreSqlContainer("postgres:18.4-alpine")
    .withUsername("postgres")
    .withPassword("123")
    .withDatabase("postgres")
    .start();

  const databaseUrl = container.getConnectionUri();

  process.env.DATABASE_URL = databaseUrl;

  console.log(
    `PostgreSQL container is up at ${databaseUrl}. Initializing schema...`,
  );

  // Execute schema init from initTest.sql

  const initSqlPath = path.resolve(process.cwd(), '../init.sql');
  const initSql = fs.readFileSync(initSqlPath, "utf8");

  process.env.DATABASE_INIT_SQL = initSql

  const sql = postgres(databaseUrl);
  await sql.unsafe(initSql);
  await sql.end();

  console.log("Database schema initialized successfully.");

  // Store container reference globally in globalThis for teardown
  globalThis.__postgresContainer__ = container;
}

export async function teardown() {
  if (globalThis.__postgresContainer__) {
    console.log("Stopping PostgreSQL container...");
    await globalThis.__postgresContainer__.stop();
    console.log("PostgreSQL container stopped.");
  }
}
