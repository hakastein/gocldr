// Generates testdata/intl_plurals.json: a matrix of Intl.PluralRules results
// across many locales x {cardinal, ordinal} x many numeric values. The Go test
// loads this and asserts the generated tables agree with the V8 (full-ICU) Intl
// implementation, which derives from the same CLDR data.
//
// Each value is emitted as a string to preserve fraction-digit width (e.g.
// "1.0" vs "1"), and the select() call is fed the matching minimum/maximum
// fraction-digit options so Intl computes the same operands the Go side will.
//
// Usage: node intl.js <out.json>

const fs = require('fs');
const outPath = process.argv[2];

const locales = [
  'en', 'ru', 'pl', 'ar', 'cy', 'fr', 'ja', 'lt', 'sl', 'he',
  'pt', 'pt-PT', 'es', 'de', 'it', 'cs', 'sk', 'uk', 'ro', 'ga',
  'gd', 'br', 'mt', 'lv', 'mk', 'is', 'fil', 'hi', 'be', 'hy',
  'ka', 'kw', 'naq', 'gv', 'tzm', 'lag', 'ksh', 'shi', 'sr', 'hr',
  'bs', 'ca', 'da', 'nl', 'fi', 'hu', 'id', 'ko', 'th', 'tr', 'vi', 'zh',
];

// Value strings with explicit fraction-digit shapes.
const intValues = [];
for (let i = 0; i <= 130; i++) intValues.push(String(i));
for (const v of [200, 201, 202, 203, 204, 205, 211, 212, 1000, 1001, 1002,
  10000, 100000, 1000000, 1000001, 11, 12, 13, 14, 21, 22, 101, 111]) {
  intValues.push(String(v));
}

const decValues = [
  '0.0', '0.1', '0.5', '1.0', '1.1', '1.5', '1.9', '2.0', '2.5', '2.7',
  '3.0', '3.5', '10.0', '10.1', '100.0', '0.00', '1.00', '1.50', '2.00',
  '0.000', '1.500', '0.3', '0.7', '11.0', '12.0', '0.2', '5.0', '7.5',
];

function fracDigits(s) {
  const dot = s.indexOf('.');
  return dot < 0 ? 0 : s.length - dot - 1;
}

const out = [];
for (const locale of locales) {
  for (const type of ['cardinal', 'ordinal']) {
    const values = type === 'cardinal' ? intValues.concat(decValues) : intValues;
    for (const vs of values) {
      const fd = fracDigits(vs);
      const n = parseFloat(vs);
      let category;
      try {
        const pr = new Intl.PluralRules(locale, {
          type,
          minimumFractionDigits: fd,
          maximumFractionDigits: fd,
        });
        category = pr.select(n);
      } catch (e) {
        continue; // skip locales Intl cannot construct
      }
      out.push({ locale, type, value: vs, minFrac: fd, maxFrac: fd, category });
    }
  }
}

fs.writeFileSync(outPath, JSON.stringify(out, null, 0));
console.error(`wrote ${out.length} intl rows to ${outPath}`);
