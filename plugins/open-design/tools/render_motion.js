#!/usr/bin/env node
/**
 * Open Design HyperFrames Motion Video Renderer for Forge ADE
 * Usage: node render_motion.js --input=motion.html --output=motion.mp4
 */
const fs = require('fs');
const path = require('path');
const { execSync, spawn } = require('child_process');

function parseArgs() {
  const args = process.argv.slice(2);
  const params = {
    input: 'motion.html',
    output: 'motion.mp4',
    fps: 30
  };

  for (const arg of args) {
    if (arg.startsWith('--input=')) params.input = arg.split('=')[1];
    else if (arg.startsWith('--output=')) params.output = arg.split('=')[1];
    else if (arg.startsWith('--fps=')) params.fps = Number(arg.split('=')[1]);
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

// 1. Locate ffmpeg and ffprobe binaries (prefers bundled static binaries, then PATH)
let ffmpegBin = 'ffmpeg';
let ffprobeBin = 'ffprobe';

try {
  const staticFfmpeg = require('ffmpeg-static');
  if (staticFfmpeg && fs.existsSync(staticFfmpeg)) {
    ffmpegBin = staticFfmpeg;
  }
} catch (e) {}

try {
  const staticFfprobe = require('ffprobe-static');
  if (staticFfprobe && staticFfprobe.path && fs.existsSync(staticFfprobe.path)) {
    ffprobeBin = staticFfprobe.path;
  }
} catch (e) {}

console.log(JSON.stringify({
  status: 'starting',
  input: inputPath,
  output: outputPath,
  ffmpeg: ffmpegBin,
  ffprobe: ffprobeBin,
  message: `Rendering HyperFrames motion composition ${params.input} to ${params.output}...`
}, null, 2));

// 2. Set up temporary composition directory required by hyperframes
const tempCompDir = path.join(path.dirname(inputPath), '.hyperframes-render-' + Date.now());
fs.mkdirSync(tempCompDir, { recursive: true });

const htmlContent = fs.readFileSync(inputPath, 'utf8');
fs.writeFileSync(path.join(tempCompDir, 'index.html'), htmlContent, 'utf8');
fs.writeFileSync(path.join(tempCompDir, 'hyperframes.json'), JSON.stringify({
  "$schema": "https://hyperframes.heygen.com/schema/hyperframes.json"
}, null, 2), 'utf8');

try {
  // Run hyperframes render with the bundled ffmpeg and ffprobe on PATH
  const extraPaths = [path.dirname(ffmpegBin), path.dirname(ffprobeBin)].filter(Boolean).join(':');
  const env = { ...process.env, CI: '1', PATH: `${extraPaths}:${process.env.PATH}` };
  
  const cmd = `bunx hyperframes render "${tempCompDir}" --output "${outputPath}"`;
  console.log(`Executing: ${cmd}`);
  
  execSync(cmd, { stdio: ['ignore', 'inherit', 'inherit'], env });
  
  console.log(JSON.stringify({
    success: true,
    outputPath,
    message: `Successfully rendered video to ${outputPath}`
  }, null, 2));
} catch (err) {
  console.log(JSON.stringify({
    success: false,
    error: err.message,
    fallback: "You can also use the 1-click 'Download Video' button directly inside Forge ADE's right sidebar viewer."
  }, null, 2));
} finally {
  // Clean up temporary composition folder
  try {
    fs.rmSync(tempCompDir, { recursive: true, force: true });
  } catch (_) {}
}
