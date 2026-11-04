import { describe, it, expect } from 'vitest';
import {
  detectSlides,
  estimateTokenCount,
  formatElapsedSeconds,
  generateArtifactFilename
} from './artifactsUtils';

describe('artifactsUtils', () => {
  describe('detectSlides', () => {
    it('returns empty array on empty html', () => {
      expect(detectSlides('')).toEqual([]);
    });

    it('returns single slide when no slide markers present', () => {
      const html = '<div><h1>Title</h1><p>Content</p></div>';
      expect(detectSlides(html)).toEqual([html]);
    });

    it('splits slides by <!-- slide --> comments', () => {
      const html = `
        <h1>Slide 1</h1>
        <!-- slide -->
        <h1>Slide 2</h1>
        <!-- slide -->
        <h1>Slide 3</h1>
      `;
      const slides = detectSlides(html);
      expect(slides).toHaveLength(3);
      expect(slides[0]).toBe('<h1>Slide 1</h1>');
      expect(slides[1]).toBe('<h1>Slide 2</h1>');
      expect(slides[2]).toBe('<h1>Slide 3</h1>');
    });

    it('detects <section class="slide"> elements', () => {
      const html = `
        <section class="slide">First</section>
        <section class="slide">Second</section>
      `;
      const slides = detectSlides(html);
      expect(slides).toHaveLength(2);
      expect(slides[0]).toContain('First');
      expect(slides[1]).toContain('Second');
    });
  });

  describe('estimateTokenCount', () => {
    it('estimates tokens using ~4 chars per token rule', () => {
      expect(estimateTokenCount('')).toBe(0);
      expect(estimateTokenCount('hello world')).toBe(3);
      expect(estimateTokenCount('a'.repeat(400))).toBe(100);
    });
  });

  describe('formatElapsedSeconds', () => {
    it('formats seconds with decimal or whole', () => {
      expect(formatElapsedSeconds(12)).toBe('12s');
      expect(formatElapsedSeconds(14.5)).toBe('14.5s');
      expect(formatElapsedSeconds(65)).toBe('1m 5s');
      expect(formatElapsedSeconds(120)).toBe('2m 0s');
    });

    it('formats minutes and hours according to human readable format', () => {
      expect(formatElapsedSeconds(242)).toBe('4m 2s');
      expect(formatElapsedSeconds(5160)).toBe('1h 26m');
      expect(formatElapsedSeconds(3600)).toBe('1h');
      expect(formatElapsedSeconds(7260)).toBe('2h 1m');
    });
  });

  describe('generateArtifactFilename', () => {
    it('generates sanitized filename with version tag', () => {
      expect(generateArtifactFilename('Interactive Chart Demo', 3, 'html'))
        .toBe('interactive-chart-demo-v3.html');
      expect(generateArtifactFilename('$$Special * Characters!$$', 1, 'svg'))
        .toBe('special-characters-v1.svg');
    });
  });
});
