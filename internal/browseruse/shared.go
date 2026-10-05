package browseruse

import (
	"encoding/json"
	"strconv"
	"strings"
)

// Shared definitions for both engines: the snapshot/ref contract, key names,
// and the page-side scripts that drive snapshot → act → verify.

type refRect struct {
	Found bool    `json:"found"`
	X     float64 `json:"x"`
	Y     float64 `json:"y"`
	W     float64 `json:"w"`
	H     float64 `json:"h"`
}

// cdpKeyMap translates friendly key names to CDP key descriptors.
var cdpKeyMap = map[string]struct {
	code      string
	key       string
	keyCode   int
	windowsVK int
}{
	"enter":      {"Enter", "Enter", 13, 13},
	"tab":        {"Tab", "Tab", 9, 9},
	"escape":     {"Escape", "Escape", 27, 27},
	"backspace":  {"Backspace", "Backspace", 8, 8},
	"delete":     {"Delete", "Delete", 46, 46},
	"space":      {"Space", " ", 32, 32},
	"arrowup":    {"ArrowUp", "ArrowUp", 38, 38},
	"arrowdown":  {"ArrowDown", "ArrowDown", 40, 40},
	"arrowleft":  {"ArrowLeft", "ArrowLeft", 37, 37},
	"arrowright": {"ArrowRight", "ArrowRight", 39, 39},
	"home":       {"Home", "Home", 36, 36},
	"end":        {"End", "End", 35, 35},
	"pageup":     {"PageUp", "PageUp", 33, 33},
	"pagedown":   {"PageDown", "PageDown", 34, 34},
}

// resolveRefScript (fmt.Sprintf %q for the ref) scrolls a snapshot ref into
// view and returns its viewport rect.
const resolveRefScript = `((ref) => {
  const el = document.querySelector('[data-fref="' + ref + '"]');
  if (!el) return JSON.stringify({found: false});
  el.scrollIntoView({block: 'center', inline: 'center'});
  const r = el.getBoundingClientRect();
  return JSON.stringify({found: true, x: r.x + r.width / 2, y: r.y + r.height / 2, w: r.width, h: r.height});
})(%q)`

// pageStatusScript returns url + title as JSON (status polling for the
// in-app engine, which has no navigation delegate).
const pageStatusScript = `JSON.stringify({url: location.href, title: document.title, ready: document.readyState})`

// insertTextScript inserts a printable character into the focused element.
const insertTextScript = `((ch) => { document.execCommand('insertText', false, ch); return 'ok'; })(%q)`

// normalizeURL applies scheme defaults: http for local hosts, https otherwise.
func normalizeURL(raw string) string {
	if strings.Contains(raw, "://") ||
		strings.HasPrefix(raw, "about:") ||
		strings.HasPrefix(raw, "data:") ||
		strings.HasPrefix(raw, "file:") {
		return raw
	}
	if strings.HasPrefix(raw, "localhost") ||
		strings.HasPrefix(raw, "127.0.0.1") ||
		strings.HasPrefix(raw, "[::1]") ||
		strings.HasPrefix(raw, "0.0.0.0") {
		return "http://" + raw
	}
	// Bare host:port (e.g. "myhost:3000") → http.
	if i := strings.Index(raw, ":"); i > 0 && !strings.Contains(raw[:i], "/") {
		if _, err := strconv.Atoi(strings.TrimSuffix(raw[i+1:], "/")); err == nil {
			return "http://" + raw
		}
	}
	return "https://" + raw
}

// pageLoadCountExpr reads the per-document load counter as a STRING (never a
// bare scalar — WKWebView's JSON serialization throws on scalar NSNumber
// results). Both engines inject the counter script at document start
// (WKUserScript on the in-app engine, Page.addScriptToEvaluateOnNewDocument
// on cdp); it survives reloads via sessionStorage, so "the page reloaded" is
// observable from JS.
const pageLoadCountExpr = `String(Number(window.__forgeLoads || 1))`

// reloadTraceExpr reads the JS-initiated reload trace (stack of each
// location.reload call, captured by loadCounterScript).
const reloadTraceExpr = `String(sessionStorage.getItem('__forgeReloadTrace') || '')`

// parseLoadCount extracts the count from the JSON-encoded result of
// pageLoadCountExpr (e.g. the raw JSON string "7"), tolerating engines that
// hand back a number or garbage.
func parseLoadCount(raw json.RawMessage) int {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		if n, err := strconv.Atoi(strings.TrimSpace(s)); err == nil && n > 0 {
			return n
		}
	}
	var n int
	if json.Unmarshal(raw, &n) == nil && n > 0 {
		return n
	}
	return 1
}

// loadCounterScript is the document-start counter injection. The in-app
// engine's ObjC copy (iab_create_impl) must stay byte-identical. It also
// wraps location.reload to leave a stack trace of JS-initiated reloads in
// sessionStorage (__forgeReloadTrace) — the storm diagnostic.
const loadCounterScript = `try{
var n=parseInt(sessionStorage.getItem('__forgeLoads')||'0')+1;
sessionStorage.setItem('__forgeLoads',String(n));
window.__forgeLoads=n;
if(!window.__forgeReloadHooked){
  window.__forgeReloadHooked=true;
  var origReload=location.reload.bind(location);
  location.reload=function(){
    try{
      var t=JSON.parse(sessionStorage.getItem('__forgeReloadTrace')||'[]');
      t.push({t:Date.now(),href:String(location.href),stack:String(new Error().stack||'').split('\n').slice(1,6).join(' | ')});
      sessionStorage.setItem('__forgeReloadTrace',JSON.stringify(t.slice(-8)));
    }catch(e){}
    return origReload();
  };
}
}catch(e){window.__forgeLoads=1;}`

// clickScript (fmt.Sprintf %q for the ref) synthesizes a full pointer + click
// event sequence on the element — works for React/Vue/etc. handlers. Returns
// the element's label afterwards so the model immediately sees the effect.
const clickScript = `((ref) => {
  const el = document.querySelector('[data-fref="' + ref + '"]');
  if (!el) return JSON.stringify({found: false});
  el.scrollIntoView({block: 'center', inline: 'center'});
  const r = el.getBoundingClientRect();
  const opts = {bubbles: true, cancelable: true, composed: true, view: window,
                clientX: r.x + r.width / 2, clientY: r.y + r.height / 2, button: 0};
  try { el.dispatchEvent(new PointerEvent('pointerdown', Object.assign({pointerId: 1, isPrimary: true, pointerType: 'mouse'}, opts))); } catch (e) {}
  el.dispatchEvent(new MouseEvent('mousedown', opts));
  try { el.dispatchEvent(new PointerEvent('pointerup', Object.assign({pointerId: 1, isPrimary: true, pointerType: 'mouse'}, opts))); } catch (e) {}
  el.dispatchEvent(new MouseEvent('mouseup', opts));
  el.click();
  const label = ((el.innerText || el.value || '') + '').replace(/\s+/g, ' ').trim().slice(0, 60);
  return JSON.stringify({found: true, clicked_label: label, href: location.href, loads: window.__forgeLoads || 1});
})(%q)`

// typeScript (refs + %q for text) focuses the element and sets its value with
// framework-compatible input/change events.
const typeScript = `((ref, text) => {
  const el = document.querySelector('[data-fref="' + ref + '"]');
  if (!el) return JSON.stringify({found: false});
  el.focus();
  if (el.isContentEditable) {
    el.textContent = text;
    el.dispatchEvent(new InputEvent('input', {bubbles: true, data: text}));
    return JSON.stringify({found: true});
  }
  const proto = el.tagName === 'TEXTAREA' ? HTMLTextAreaElement.prototype : HTMLInputElement.prototype;
  const setter = Object.getOwnPropertyDescriptor(proto, 'value').set;
  setter.call(el, text);
  el.dispatchEvent(new Event('input', {bubbles: true}));
  el.dispatchEvent(new Event('change', {bubbles: true}));
  return JSON.stringify({found: true});
})(%q, %q)`

// submitScript presses Enter on the active element and submits the owning
// form when one exists.
const submitScript = `(() => {
  const el = document.activeElement;
  if (el) {
    const opts = {bubbles: true, cancelable: true, key: 'Enter', code: 'Enter', keyCode: 13, which: 13};
    el.dispatchEvent(new KeyboardEvent('keydown', opts));
    el.dispatchEvent(new KeyboardEvent('keypress', opts));
    el.dispatchEvent(new KeyboardEvent('keyup', opts));
    if (el.form && typeof el.form.requestSubmit === 'function') { el.form.requestSubmit(); }
    else if (el.form) { el.form.submit(); }
  }
  return 'ok';
})()`

// keyScript (fmt.Sprintf %q for name/key/code) dispatches a keyboard event on
// the active element.
const keyScript = `((name, code, keyCode) => {
  const target = document.activeElement || document.body;
  const base = {bubbles: true, cancelable: true, key: name, code: code};
  const down = Object.assign({}, base, {keyCode: keyCode, which: keyCode});
  target.dispatchEvent(new KeyboardEvent('keydown', down));
  target.dispatchEvent(new KeyboardEvent('keypress', down));
  target.dispatchEvent(new KeyboardEvent('keyup', down));
  return 'ok';
})(%q, %q, %d)`

// snapshotScript walks the DOM and emits a compact YAML-ish outline of
// headings, text, and interactive elements. Interactive elements get a
// data-fref attribute that Click/Type resolve later — the same ref pattern
// ZCode's browser-use plugin and Playwright's aria snapshot use.
const snapshotScript = `(() => {
  const INTERACTIVE = 'a[href],button,input,textarea,select,summary,[contenteditable=""],[contenteditable="true"],[role="button"],[role="link"],[role="checkbox"],[role="radio"],[role="tab"],[role="menuitem"],[role="option"],[role="switch"],[role="textbox"],[role="combobox"],[onclick]';
  let n = 0, out = [];
  const clean = (s, max) => (s || '').replace(/\s+/g, ' ').trim().slice(0, max || 90);
  const roleOf = (el) => {
    const r = el.getAttribute('role');
    if (r) return r;
    const tag = el.tagName.toLowerCase();
    if (tag === 'a') return 'link';
    if (tag === 'button') return 'button';
    if (tag === 'input') {
      const t = (el.type || 'text').toLowerCase();
      if (t === 'checkbox') return 'checkbox';
      if (t === 'radio') return 'radio';
      if (['submit','button','reset'].includes(t)) return 'button';
      return 'textbox';
    }
    if (tag === 'textarea') return 'textbox';
    if (tag === 'select') return 'combobox';
    if (tag === 'summary') return 'summary';
    return tag;
  };
  const visible = (el) => {
    const r = el.getBoundingClientRect();
    if (r.width < 2 && r.height < 2) return false;
    const st = getComputedStyle(el);
    return st.visibility !== 'hidden' && st.display !== 'none';
  };
  const walk = (node, depth) => {
    if (!node || out.length > 400) return;
    for (const el of node.children) {
      const tag = el.tagName.toLowerCase();
      if (['script','style','noscript','svg','path','template','iframe'].includes(tag)) continue;
      if (!visible(el)) continue;
      const isInteractive = el.matches(INTERACTIVE);
      const isHeading = /^h[1-6]$/.test(tag);
      const isImg = tag === 'img' && el.getAttribute('alt');
      const ownText = Array.from(el.childNodes).filter(c => c.nodeType === 3).map(c => c.textContent).join(' ');
      if (isInteractive) {
        const ref = 'e' + (++n);
        el.setAttribute('data-fref', ref);
        const role = roleOf(el);
        let label = clean(el.getAttribute('aria-label') || el.getAttribute('placeholder') || el.innerText || el.value || ownText || el.getAttribute('title') || el.getAttribute('name'));
        if (role === 'textbox' || role === 'combobox') {
          const val = clean(el.value || '', 60);
          out.push('- ref=' + ref + ': ' + role + (label ? ' "' + label + '"' : '') + (val ? ' value="' + val + '"' : ' [empty]'));
        } else if (role === 'checkbox' || role === 'radio' || role === 'switch') {
          out.push('- ref=' + ref + ': ' + role + (label ? ' "' + label + '"' : '') + (el.checked ? ' [checked]' : ' [unchecked]'));
        } else if (role === 'link' && el.getAttribute('href')) {
          out.push('- ref=' + ref + ': link "' + (label || '[no text]') + '" -> ' + clean(el.getAttribute('href'), 120));
        } else {
          out.push('- ref=' + ref + ': ' + role + (label ? ' "' + label + '"' : ' [no text]'));
        }
      } else if (isHeading) {
        out.push('#' + tag[1] + ' ' + clean(el.innerText));
      } else if (isImg) {
        out.push('- image "' + clean(el.getAttribute('alt'), 60) + '"');
      } else if (tag === 'p' || tag === 'li' || tag === 'blockquote') {
        const t = clean(el.innerText, 200);
        if (t) out.push((tag === 'li' ? '- ' : '') + t);
      } else if (ownText && ownText.trim().length > 2 && el.children.length === 0) {
        out.push(clean(ownText, 160));
      }
      if (tag !== 'select' && depth < 40) walk(el, depth + 1);
    }
  };
  if (document.body) walk(document.body, 0);
  const title = document.title || location.href;
  return 'page: ' + title + ' (load ' + (window.__forgeLoads || 1) + ')\n' + (out.length ? out.join('\n') : '(no interactive elements found)');
})()`

// pickElementScript arms the element picker (element-select-for-chat in the
// viewer). Returns "armed" immediately; the user's next click stores the
// picked element as JSON in window.__forgePickResult (engines poll for it).
const pickElementScript = `(() => {
  if (window.__forgePicker) return 'already-armed';
  window.__forgePickResult = undefined;
  let lastOutlined = null;
  const outline = (el, on) => {
    if (lastOutlined) { lastOutlined.style.outline = ''; lastOutlined.style.outlineOffset = ''; }
    lastOutlined = null;
    if (on && el) { el.style.outline = '2px solid #6ea8fe'; el.style.outlineOffset = '-2px'; lastOutlined = el; }
  };
  const move = (ev) => outline(ev.target, true);
  const cleanup = () => {
    document.removeEventListener('mousemove', move, true);
    document.removeEventListener('click', click, true);
    outline(null, false);
    delete window.__forgePicker;
  };
  const click = (ev) => {
    ev.preventDefault(); ev.stopPropagation();
    const el = ev.target;
    const path = [];
    let n = el;
    while (n && n.nodeType === 1 && path.length < 6) {
      let sel = n.tagName.toLowerCase();
      if (n.id) sel += '#' + n.id;
      else if (n.getAttribute('data-testid')) sel += '[data-testid="' + n.getAttribute('data-testid') + '"]';
      else if (n.getAttribute('name')) sel += '[name="' + n.getAttribute('name') + '"]';
      path.push(sel);
      n = n.parentElement;
    }
    window.__forgePickResult = JSON.stringify({
      selector: path.join(' > '),
      tag: el.tagName.toLowerCase(),
      text: (el.innerText || el.value || '').replace(/\s+/g, ' ').trim().slice(0, 120),
      role: el.getAttribute('role') || '',
      rect: ((r) => ({x: r.x, y: r.y, w: r.width, h: r.height}))(el.getBoundingClientRect())
    });
    cleanup();
  };
  document.addEventListener('mousemove', move, true);
  document.addEventListener('click', click, true);
  window.__forgePicker = cleanup;
  setTimeout(() => {
    if (window.__forgePicker === cleanup && window.__forgePickResult === undefined) {
      window.__forgePickResult = JSON.stringify({cancelled: true});
      cleanup();
    }
  }, 30000);
  return 'armed';
})()`
