#!/usr/bin/env bun
// Sync repo skills/ into site/public/.well-known/skills/ for npx skills distribution.
//
//   bun site/scripts/sync-skills.mjs
//
// Reads skills/<skill-name>/SKILL.md, parses YAML frontmatter (name & description),
// copies the skill directory into site/public/.well-known/skills/<skill-name>/,
// and generates site/public/.well-known/skills/index.json per RFC 8615.
// Also ensures site/public/.nojekyll exists so GitHub Pages does not ignore dot-folders.

import {
  mkdirSync,
  rmSync,
  cpSync,
  existsSync,
  readFileSync,
  writeFileSync,
  readdirSync,
} from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';

const root = join(dirname(fileURLToPath(import.meta.url)), '..', '..');
const skillsSrc = join(root, 'skills');
const publicDir = join(root, 'site', 'public');
const wellKnownSkillsDir = join(publicDir, '.well-known', 'skills');
const nojekyllFile = join(publicDir, '.nojekyll');

// 1. Ensure .nojekyll exists
if (!existsSync(nojekyllFile)) {
  writeFileSync(nojekyllFile, '', 'utf8');
}

// 2. Clean and recreate .well-known/skills
rmSync(wellKnownSkillsDir, { recursive: true, force: true });
mkdirSync(wellKnownSkillsDir, { recursive: true });

const manifest = {
  skills: [],
};

if (existsSync(skillsSrc)) {
  const entries = readdirSync(skillsSrc, { withFileTypes: true });

  for (const entry of entries) {
    if (!entry.isDirectory()) continue;
    const skillName = entry.name;
    const skillDir = join(skillsSrc, skillName);
    const skillFile = join(skillDir, 'SKILL.md');

    if (!existsSync(skillFile)) continue;

    const content = readFileSync(skillFile, 'utf8');
    const frontmatterMatch = content.match(/^---\r?\n([\s\S]*?)\r?\n---/);
    if (!frontmatterMatch) continue;

    const fm = frontmatterMatch[1];
    let name = skillName;
    let description = '';

    const nameMatch = fm.match(/^name:\s*(.+)$/m);
    if (nameMatch) {
      name = nameMatch[1].trim();
    }

    const descMatch = fm.match(/^description:\s*(?:>-\s*)?([\s\S]*?)(?:\n\w+:|$)/m);
    if (descMatch) {
      description = descMatch[1].replace(/\n\s+/g, ' ').trim();
    }

    // List all files inside the skill directory
    const files = [];
    function collectFiles(dir, prefix = '') {
      for (const item of readdirSync(dir, { withFileTypes: true })) {
        const itemRel = prefix ? `${prefix}/${item.name}` : item.name;
        if (item.isDirectory()) {
          collectFiles(join(dir, item.name), itemRel);
        } else if (item.isFile()) {
          files.push(itemRel);
        }
      }
    }
    collectFiles(skillDir);

    // Copy skill folder into .well-known/skills/<name>
    const destSkillDir = join(wellKnownSkillsDir, skillName);
    mkdirSync(destSkillDir, { recursive: true });
    cpSync(skillDir, destSkillDir, { recursive: true });

    manifest.skills.push({
      name,
      description,
      files,
    });
  }
}

// 3. Write index.json
const indexPath = join(wellKnownSkillsDir, 'index.json');
writeFileSync(indexPath, JSON.stringify(manifest, null, 2) + '\n', 'utf8');

console.log(
  `Synced ${manifest.skills.length} skill(s) to ${wellKnownSkillsDir} and generated index.json`
);
