#!/usr/bin/env node
// Generates golden fixtures from Node's Intl.NumberFormat (full-ICU) for the
// Go number package to assert against. Run with Node 22 (full ICU):
//
//   node number/internal/gen/fixtures.js
//     (from any working directory — always writes to
//      number/testdata/intl_numbers.json relative to this script)
//
// Normally invoked via `go generate ./number/...` which runs the
// //go:generate directive in number.go.
//
// Each fixture is {locale, value, options, expected}. The Go test loads these
// and asserts number.Format(locale, value, options) == expected.

'use strict';

const fs = require('fs');
const path = require('path');

const locales = [
  'en', 'en-IN', 'en-GB', 'de', 'fr', 'fr-CH', 'ru', 'ar', 'ar-EG', 'hi',
  'ja', 'pl', 'pt-BR', 'pt-PT', 'es', 'es-MX', 'it', 'nl', 'sv', 'tr',
  'fa', 'bn', 'th', 'zh', 'zh-Hant', 'ko', 'cs', 'da', 'fi', 'nb',
  'he', 'uk', 'ro', 'hu', 'el', 'id', 'vi', 'ca', 'hr', 'sk',
];

const currencies = ['USD', 'EUR', 'JPY', 'INR', 'BHD', 'GBP', 'RUB', 'CNY', 'KWD', 'CLP'];

const values = [
  0, 1, -1, 1234.5, -1234.567, 0.005, 1234567.891, 1000000, 0.1, 12.3456,
  -0.5, 999999.999, 0.00012345, 9999.5, 100, 0.9995, 2.5, 3.5, 1234.0, 5,
];

const fixtures = [];

function add(locale, value, options) {
  let expected;
  try {
    expected = new Intl.NumberFormat(locale, options).format(value);
  } catch (e) {
    return; // skip option combos ICU rejects
  }
  fixtures.push({ locale, value, options, expected });
}

// 1) Plain decimal across all locales and values.
for (const l of locales) {
  for (const v of values) {
    add(l, v, { style: 'decimal' });
  }
}

// 2) decimal option variations on a representative subset of locales.
const optLocales = ['en', 'de', 'fr', 'ru', 'ar', 'hi', 'ja', 'en-IN', 'fa', 'bn', 'th', 'pl', 'es', 'pt-BR'];
const decimalOptionSets = [
  { minimumFractionDigits: 2 },
  { maximumFractionDigits: 0 },
  { minimumFractionDigits: 2, maximumFractionDigits: 4 },
  { minimumSignificantDigits: 3 },
  { maximumSignificantDigits: 3 },
  { minimumSignificantDigits: 2, maximumSignificantDigits: 5 },
  { useGrouping: false },
  { useGrouping: true },
  { useGrouping: 'always' },
  { useGrouping: 'auto' },
  { useGrouping: 'min2' },
  // ES2023 legacy quirk: the STRING 'true' is an alias of 'auto', not 'always'.
  { useGrouping: 'true' },
  { minimumIntegerDigits: 4 },
  { minimumFractionDigits: 0, maximumFractionDigits: 6 },
];
for (const l of optLocales) {
  for (const v of values) {
    for (const o of decimalOptionSets) {
      add(l, v, Object.assign({ style: 'decimal' }, o));
    }
  }
}

// 3) Percent.
for (const l of optLocales) {
  for (const v of [0, 0.1, 0.1234, 0.5, 1, -0.25, 0.005, 0.9995, 1.5]) {
    add(l, v, { style: 'percent' });
    add(l, v, { style: 'percent', minimumFractionDigits: 1 });
    add(l, v, { style: 'percent', maximumFractionDigits: 2 });
  }
}

// 4) Currency, all display modes.
const currLocales = ['en', 'de', 'fr', 'ru', 'ja', 'ar', 'ar-EG', 'hi', 'en-IN', 'pt-BR', 'es', 'it', 'nl', 'zh'];
const displays = ['symbol', 'narrowSymbol', 'code', 'name'];
for (const l of currLocales) {
  for (const c of currencies) {
    for (const v of [0, 1, 1234.5, -1234.567, 1000000, 0.005, 5]) {
      for (const d of displays) {
        add(l, v, { style: 'currency', currency: c, currencyDisplay: d });
      }
    }
  }
}

// 5) Currency with explicit fraction overrides.
for (const l of ['en', 'de', 'ja']) {
  for (const c of ['USD', 'JPY', 'BHD']) {
    add(l, 1234.5, { style: 'currency', currency: c, minimumFractionDigits: 0 });
    add(l, 1234.5, { style: 'currency', currency: c, maximumFractionDigits: 4 });
  }
}

const outFile = path.join(__dirname, '..', '..', 'testdata', 'intl_numbers.json');
fs.writeFileSync(outFile, JSON.stringify(fixtures, null, 0));
process.stderr.write(`gen: wrote ${outFile} (${fixtures.length} fixtures)\n`);
