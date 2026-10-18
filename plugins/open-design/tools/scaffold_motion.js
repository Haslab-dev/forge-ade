#!/usr/bin/env node
/**
 * Open Design HyperFrames Motion Scaffolder for Forge ADE
 * Usage: node scaffold_motion.js --target=my-motion.html --duration=10 --title="Product Reveal"
 */
const fs = require('fs');
const path = require('path');

function parseArgs() {
  const args = process.argv.slice(2);
  const params = {
    target: 'motion.html',
    title: 'HyperFrames Motion',
    duration: '8'
  };

  for (const arg of args) {
    if (arg.startsWith('--target=')) params.target = arg.split('=')[1];
    else if (arg.startsWith('--title=')) params.title = arg.split('=')[1];
    else if (arg.startsWith('--duration=')) params.duration = arg.split('=')[1];
  }
  return params;
}

const params = parseArgs();
const templatePath = path.join(__dirname, '..', 'templates', 'hyperframes-template.html');

let content = fs.readFileSync(templatePath, 'utf8');

// Replace title & duration tokens
content = content.replace(/<title>.*?<\/title>/, `<title>${params.title}</title>`);
content = content.replace(/data-duration=".*?"/, `data-duration="${params.duration}"`);
content = content.replace(/max="8"/, `max="${params.duration}"`);
content = content.replace(/1920×1080 · 8s/, `1920×1080 · ${params.duration}s`);
content = content.replace(/duration: 8/g, `duration: ${params.duration}`);

const outputPath = path.resolve(process.cwd(), params.target);
fs.writeFileSync(outputPath, content, 'utf8');

console.log(JSON.stringify({
  success: true,
  path: outputPath,
  duration: Number(params.duration),
  width: 1920,
  height: 1080,
  message: `Scaffolded Open Design HyperFrames motion at ${outputPath}`
}, null, 2));
