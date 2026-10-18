#!/usr/bin/env node
/**
 * Open Design Deck Scaffolder for Forge ADE
 * Usage: node scaffold_deck.js --target=my-deck.html --theme=klein-blue --title="Quarterly Review"
 */
const fs = require('fs');
const path = require('path');

function parseArgs() {
  const args = process.argv.slice(2);
  const params = {
    target: 'deck.html',
    title: 'Presentation Deck',
    theme: 'klein-blue'
  };

  for (const arg of args) {
    if (arg.startsWith('--target=')) params.target = arg.split('=')[1];
    else if (arg.startsWith('--title=')) params.title = arg.split('=')[1];
    else if (arg.startsWith('--theme=')) params.theme = arg.split('=')[1];
  }
  return params;
}

const THEMES = {
  'klein-blue': {
    accent: '#002FA7',
    contrast: '#ffffff',
    bg: '#fafaf8',
    fg: '#0a0a0a',
    surface: '#f0f0ec'
  },
  'lemon': {
    accent: '#FFD500',
    contrast: '#0a0a0a',
    bg: '#f7f5ee',
    fg: '#0a0a0a',
    surface: '#ece9df'
  },
  'neon': {
    accent: '#C5E803',
    contrast: '#0a0a0a',
    bg: '#f7f5ee',
    fg: '#0a0a0a',
    surface: '#ece9df'
  },
  'safety-orange': {
    accent: '#FF6B35',
    contrast: '#ffffff',
    bg: '#f7f5ee',
    fg: '#0a0a0a',
    surface: '#ece9df'
  }
};

const params = parseArgs();
const theme = THEMES[params.theme] || THEMES['klein-blue'];
const templatePath = path.join(__dirname, '..', 'templates', 'deck-template.html');

let content = fs.readFileSync(templatePath, 'utf8');

// Replace title & theme tokens
content = content.replace(/<title>.*?<\/title>/, `<title>${params.title}</title>`);
content = content.replace(/--accent:\s*#[0-9a-fA-F]+;/, `--accent: ${theme.accent};`);
content = content.replace(/--accent-contrast:\s*#[0-9a-fA-F]+;/, `--accent-contrast: ${theme.contrast};`);
content = content.replace(/--bg:\s*#[0-9a-fA-F]+;/, `--bg: ${theme.bg};`);
content = content.replace(/--fg:\s*#[0-9a-fA-F]+;/, `--fg: ${theme.fg};`);
content = content.replace(/--surface:\s*#[0-9a-fA-F]+;/, `--surface: ${theme.surface};`);

const outputPath = path.resolve(process.cwd(), params.target);
fs.writeFileSync(outputPath, content, 'utf8');

console.log(JSON.stringify({
  success: true,
  path: outputPath,
  theme: params.theme,
  slides: 3,
  message: `Scaffolded Open Design deck at ${outputPath}`
}, null, 2));
