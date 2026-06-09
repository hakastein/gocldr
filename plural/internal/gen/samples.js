// Generates testdata/cldr_samples.json from the CLDR plurals/ordinals data.
//
// For every (type, locale, category) it expands the @integer and @decimal
// sample lists from the CLDR rule strings into concrete numeric strings (with
// their canonical fraction-digit form preserved) so the Go test can assert each
// sample maps back to its declared category.
//
// Usage:
//   node samples.js <plurals.json> <ordinals.json> <out.json>

const fs = require('fs');
const path = require('path');

// Two call forms:
//   node samples.js <plurals.json> <ordinals.json> <out.json>   (explicit paths)
//   node samples.js <out.json>                                   (paths from $CLDR_DATA)
// In the second form the CLDR base is $CLDR_DATA (the pinned Docker toolchain's
// node_modules), falling back to the checked-in host copy so host runs still work.
let plularsPath, ordinalsPath, outPath;
if (process.argv.length >= 5) {
  plularsPath = process.argv[2];
  ordinalsPath = process.argv[3];
  outPath = process.argv[4];
} else {
  const base = process.env.CLDR_DATA ||
    path.resolve(__dirname, '../../../../.reference/cldr-data/node_modules');
  plularsPath = path.join(base, 'cldr-core', 'supplemental', 'plurals.json');
  ordinalsPath = path.join(base, 'cldr-core', 'supplemental', 'ordinals.json');
  outPath = process.argv[2];
}

function fracDigits(s) {
  const dot = s.indexOf('.');
  return dot < 0 ? 0 : s.length - dot - 1;
}

// Expand a single sample token. Tokens are either a value ("1", "1.5",
// "1.0000001c6") or an inclusive range ("2~16", "0.0~1.5"). For ranges we emit
// a handful of representative points: endpoints and a couple in between,
// preserving the fraction-digit width of the endpoints.
function expandToken(tok) {
  tok = tok.trim();
  if (tok === '' || tok === '…' || tok === '...') return [];
  const tilde = tok.indexOf('~');
  if (tilde < 0) return [tok];

  const lo = tok.slice(0, tilde).trim();
  const hi = tok.slice(tilde + 1).trim();

  // Compact-exponent samples (e.g. 1c3) inside ranges are rare; keep endpoints.
  if (lo.includes('c') || hi.includes('c') || lo.includes('e') || hi.includes('e')) {
    return [lo, hi];
  }

  const fd = Math.max(fracDigits(lo), fracDigits(hi));
  const loN = parseFloat(lo);
  const hiN = parseFloat(hi);
  const step = fd === 0 ? 1 : Math.pow(10, -fd);

  const out = new Set();
  const fmt = (x) => x.toFixed(fd);
  out.add(fmt(loN));
  out.add(fmt(hiN));
  // a few interior points
  const span = hiN - loN;
  for (let k = 1; k <= 3; k++) {
    const v = loN + (span * k) / 4;
    // snap to step grid
    const snapped = Math.round(v / step) * step;
    if (snapped > loN && snapped < hiN) out.add(fmt(snapped));
  }
  return [...out];
}

function parseSamples(ruleStr) {
  // ruleStr looks like: "<cond> @integer 1, 2~5, … @decimal 0.0~1.5, …"
  const out = [];
  const reType = /@(integer|decimal)([^@]*)/g;
  let m;
  while ((m = reType.exec(ruleStr)) !== null) {
    const body = m[2];
    for (const part of body.split(',')) {
      for (const v of expandToken(part)) {
        if (v !== '') out.push(v);
      }
    }
  }
  return out;
}

function build(path, sectionKey, type, acc) {
  const section = JSON.parse(fs.readFileSync(path, 'utf8')).supplemental[sectionKey];
  for (const loc of Object.keys(section)) {
    const rules = section[loc];
    for (const k of Object.keys(rules)) {
      const cat = k.replace('pluralRule-count-', '');
      for (const sample of parseSamples(rules[k])) {
        acc.push({ type, locale: loc, category: cat, value: sample });
      }
    }
  }
}

const acc = [];
build(plularsPath, 'plurals-type-cardinal', 'cardinal', acc);
build(ordinalsPath, 'plurals-type-ordinal', 'ordinal', acc);

fs.writeFileSync(outPath, JSON.stringify(acc, null, 0));
console.error(`wrote ${acc.length} samples to ${outPath}`);
