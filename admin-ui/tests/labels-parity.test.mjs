import {test} from "node:test";
import assert from "node:assert/strict";
import {readdirSync, readFileSync} from "node:fs";
import {fileURLToPath} from "node:url";
import path from "node:path";

// Every *-labels.mjs module exposes <name>Labels(language) returning a frozen
// {en,zh} dictionary pair. Bilingual strings must keep identical key
// structure so a page can never render a missing label in one language.

function keyStructure(value, prefix, into) {
  for (const [key, child] of Object.entries(value)) {
    const trail = prefix ? `${prefix}.${key}` : key;
    if (child && typeof child === "object") keyStructure(child, trail, into);
    else into.push(trail);
  }
}

function deepStructure(en, zh) {
  const enKeys = [], zhKeys = [];
  keyStructure(en, "", enKeys);
  keyStructure(zh, "", zhKeys);
  return {enKeys: enKeys.sort(), zhKeys: zhKeys.sort()};
}

function validateLeaves(value, trail, language) {
  for (const [key, child] of Object.entries(value)) {
    const path = trail ? `${trail}.${key}` : key;
    if (child && typeof child === "object") validateLeaves(child, path, language);
    else assert.ok(typeof child === "function" || typeof child === "string" && child.trim().length > 0, `${language} catalog leaf ${path} must be non-empty text or a formatter`);
  }
}

test("label catalogs keep identical en/zh key structure", async () => {
  const directory = path.join(path.dirname(fileURLToPath(import.meta.url)), "..", "src");
  const modules = readdirSync(directory).filter(name => name.endsWith("-labels.mjs"));
  const sourceNames = readdirSync(directory).filter(name => /\.(jsx|mjs)$/.test(name));
  assert.ok(modules.length >= 5, `expected the labels modules to be present, found: ${modules.join(", ")}`);
  let catalogs = 0;
  for (const name of modules) {
    const module = await import(`../src/${name}`);
    for (const [exportName, fn] of Object.entries(module)) {
      // Domain logic shares some filenames (queue-labels.mjs); only
      // <name>Labels language-catalog providers participate in the check.
      if (typeof fn !== "function" || !exportName.endsWith("Labels")) continue;
      const en = fn("en"), zh = fn("zh");
      if (typeof en !== "object" || en === null || typeof zh !== "object" || zh === null) continue;
      catalogs++;
      const {enKeys, zhKeys} = deepStructure(en, zh);
      assert.deepEqual(zhKeys, enKeys, `${name}:${exportName} en/zh key mismatch`);
      validateLeaves(en, "", `${name}:${exportName}:en`);
      validateLeaves(zh, "", `${name}:${exportName}:zh`);

      const importNeedle = `./${name}`;
      const consumers = sourceNames.filter(sourceName => sourceName !== name && readFileSync(path.join(directory, sourceName), "utf8").includes(importNeedle));
      assert.ok(consumers.length > 0, `${name} has no feature owner/importer`);
      const consumerSource = [readFileSync(path.join(directory, name), "utf8"), ...consumers.map(sourceName => readFileSync(path.join(directory, sourceName), "utf8"))].join("\n");
      for (const key of Object.keys(en)) {
        const escaped = key.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
        assert.match(consumerSource, new RegExp(`(?:\\.${escaped}\\b|\\[\\s*["']${escaped}["']\\s*\\])`), `${name}:${exportName}.${key} is not referenced by its feature owner`);
      }
    }
  }
  assert.ok(catalogs >= 8, `expected the label catalogs to be checked, found ${catalogs}`);
});

test("login message catalog keeps identical en/zh key structure", async () => {
  const {loginMessages} = await import("../src/login-messages.mjs");
  const {enKeys, zhKeys} = deepStructure(loginMessages.en, loginMessages.zh);
  assert.deepEqual(zhKeys, enKeys);
  for (const key of enKeys) assert.ok(key.length > 0);
});
