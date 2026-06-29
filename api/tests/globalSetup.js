import { PostgreSqlContainer } from "@testcontainers/postgresql";
import postgres from "postgres";
import fs from "fs";

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
  const initSqlUrl = new URL("./../../init.sql", import.meta.url);
  const initSql = fs.readFileSync(initSqlUrl, "utf8");

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
