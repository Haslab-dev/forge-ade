// @test.ts — index feature coverage
// Variables, functions, class, interface, type, enum, json, import/export.

export const appName = "forge-ade";
const config = { env: "dev", debug: true, port: 3000 };
const { env } = config;

export function makeUser(name: string, age = 30) {
  return { name, age, active: true };
}
const multiply = (a: number, b: number) => a * b;

export class Database {
  name = "forge";
  tables: string[] = [];
  connect(): Promise<void> {
    return Promise.resolve();
  }
  query(sql: string): any[] {
    return [];
  }
}

export interface AppConfig {
  env: string;
  debug: boolean;
  onReady(): void;
}
export type Point = { x: number; y: number };
export enum Color {
  Red,
  Green,
  Blue,
}

const serverJson = {
  name: "api",
  endpoints: { health: "/health", admin: { path: "/admin" } },
  tags: ["api", "prod"],
};
const services = [{ name: "auth", port: 8080 }];

import { readFileSync } from "fs";
import { useState, useEffect, useCallback } from "react";
export default appName;

// ── member completion — ketik `x.` cek popup ─────────────
const db = new Database("localhost");
const cfg = makeUser("gauss");
const user = serverJson;
const opts: AppConfig = { env: "dev", debug: false };
const point: Point = { x: 1, y: 2 };

// db.    → name, tables, connect, query
// cfg.   → name, age, active
// user.  → name, endpoints, tags      |  user.endpoints. → health, admin
// opts.  → env, debug, onReady        |  Color. → Red, Green, Blue
// point. → x, y                       |  services[0]. → name, port
// config. → env, debug, port          |  nested?. → pake `a.b.c.` chain
