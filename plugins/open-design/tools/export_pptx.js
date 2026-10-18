#!/usr/bin/env node
/**
 * Open Design PPTX Exporter for Forge ADE
 * Converts HTML slide decks into native PowerPoint .pptx decks.
 * Usage: node export_pptx.js --input=deck.html --output=presentation.pptx
 */
const fs = require('fs');
const path = require('path');

function parseArgs() {
  const args = process.argv.slice(2);
  const params = {
    input: 'deck.html',
    output: 'presentation.pptx'
  };

  for (const arg of args) {
    if (arg.startsWith('--input=')) params.input = arg.split('=')[1];
    else if (arg.startsWith('--output=')) params.output = arg.split('=')[1];
  }
  return params;
}

const params = parseArgs();
const inputPath = path.resolve(process.cwd(), params.input);
const outputPath = path.resolve(process.cwd(), params.output);

if (!fs.existsSync(inputPath)) {
  console.error(JSON.stringify({ error: `Input file not found: ${inputPath}` }));
  process.exit(1);
}

const html = fs.readFileSync(inputPath, 'utf8');

// Lightweight HTML slide parser for Open Design decks
function extractSlides(htmlContent) {
  const slideRegex = /<section[^>]*class="[^"]*slide[^"]*"[^>]*>([\s\S]*?)<\/section>/gi;
  const slides = [];
  let match;
  while ((match = slideRegex.exec(htmlContent)) !== null) {
    const raw = match[1];
    const titleMatch = raw.match(/<h[12][^>]*>([\s\S]*?)<\/h[12]>/i);
    const subtitleMatch = raw.match(/<p[^>]*>([\s\S]*?)<\/p>/i);
    const tagMatch = raw.match(/class="[^"]*tag[^"]*"[^>]*>([\s\S]*?)<\/[a-z]+>/i);
    const isAccentBg = match[0].includes('accent-bg');

    slides.push({
      title: titleMatch ? titleMatch[1].replace(/<br\s*\/?>/gi, ' ').replace(/<[^>]+>/g, '').trim() : '',
      subtitle: subtitleMatch ? subtitleMatch[1].replace(/<[^>]+>/g, '').trim() : '',
      tag: tagMatch ? tagMatch[1].replace(/<[^>]+>/g, '').trim() : '',
      isAccentBg,
      rawContent: raw
    });
  }
  return slides;
}

const slides = extractSlides(html);

// Check if pptxgenjs is available
let PptxGenJS;
try {
  PptxGenJS = require('pptxgenjs');
} catch (e) {
  // PptxGenJS not installed yet in project
  console.log(JSON.stringify({
    success: false,
    needsInstall: true,
    slideCount: slides.length,
    message: "pptxgenjs is required for native .pptx export. Run 'npm install pptxgenjs' or 'bun add pptxgenjs' to enable PPTX output. Meanwhile, HTML decks support instant 1-click PDF export via standard browser print."
  }, null, 2));
  process.exit(0);
}

async function generatePptx() {
  const pres = new PptxGenJS();
  pres.layout = 'LAYOUT_16x9';

  for (const s of slides) {
    const slide = pres.addSlide();
    if (s.isAccentBg) {
      slide.background = { color: '002FA7' };
      if (s.tag) {
        slide.addText(s.tag, { x: 0.8, y: 0.6, fontSize: 11, fontFace: 'Courier New', color: 'FFFFFF' });
      }
      if (s.title) {
        slide.addText(s.title, { x: 0.8, y: 2.0, w: 11.5, fontSize: 44, bold: true, color: 'FFFFFF' });
      }
      if (s.subtitle) {
        slide.addText(s.subtitle, { x: 0.8, y: 4.0, w: 10.0, fontSize: 18, color: 'E0E0E0' });
      }
    } else {
      slide.background = { color: 'FAFAF8' };
      if (s.tag) {
        slide.addText(s.tag, { x: 0.8, y: 0.6, fontSize: 11, fontFace: 'Courier New', color: '002FA7' });
      }
      if (s.title) {
        slide.addText(s.title, { x: 0.8, y: 1.2, w: 11.5, fontSize: 32, bold: true, color: '0A0A0A' });
      }
      if (s.subtitle) {
        slide.addText(s.subtitle, { x: 0.8, y: 2.2, w: 11.0, fontSize: 15, color: '6B6964' });
      }
    }
  }

  await pres.writeFile({ fileName: outputPath });
  console.log(JSON.stringify({
    success: true,
    outputPath,
    slides: slides.length,
    message: `Exported ${slides.length} slides to ${outputPath}`
  }, null, 2));
}

generatePptx().catch(err => {
  console.error(JSON.stringify({ error: err.message }));
  process.exit(1);
});
