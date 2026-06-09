// fixtures.js generates golden Intl.DateTimeFormat outputs for the Go test
// suite. Run with full-ICU node (v22):
//
//	node cldr/datetime/internal/gen/fixtures.js > cldr/datetime/testdata/intl_dates.json
//
// Every case fixes timeZone:'UTC' so the output is deterministic.

const locales = [
  "en", "en-GB", "de", "fr", "ru", "ja", "ar", "zh", "es", "it",
  "pt-BR", "ko", "fa", "hi", "nl", "pl", "tr", "sv", "th", "he",
  "cs", "el", "uk", "vi", "id", "ro", "hu", "fi", "da", "nb",
];

// Fixed UTC instants (year, monthIndex, day, hour, min, sec, ms).
const dates = [
  Date.UTC(2021, 0, 5, 15, 4, 5, 123),   // single-digit month/day, PM
  Date.UTC(2023, 10, 23, 9, 7, 2, 0),    // double-digit month/day, AM
  Date.UTC(1999, 6, 1, 0, 0, 0, 0),      // midnight, July 1
  Date.UTC(2000, 11, 31, 23, 59, 59, 0), // year boundary, late PM
  Date.UTC(2024, 1, 29, 12, 30, 0, 0),   // leap day, noon
];

const styleVals = ["full", "long", "medium", "short"];

const optionSets = [];

// dateStyle / timeStyle combinations: date-only, time-only, and both.
for (const ds of styleVals) {
  optionSets.push({ tag: "dateStyle:" + ds, opts: { dateStyle: ds } });
}
for (const ts of styleVals) {
  optionSets.push({ tag: "timeStyle:" + ts, opts: { timeStyle: ts } });
}
for (const ds of styleVals) {
  for (const ts of styleVals) {
    optionSets.push({
      tag: "both:" + ds + "/" + ts,
      opts: { dateStyle: ds, timeStyle: ts },
    });
  }
}

// Component option combinations.
const comp = [
  ["comp:ymd-num", { year: "numeric", month: "numeric", day: "numeric" }],
  ["comp:ymd-long", { year: "numeric", month: "long", day: "numeric" }],
  ["comp:ymd-short", { year: "numeric", month: "short", day: "numeric" }],
  ["comp:wymd-long", { weekday: "long", year: "numeric", month: "long", day: "numeric" }],
  ["comp:md-short", { month: "short", day: "numeric" }],
  ["comp:md-long", { month: "long", day: "numeric" }],
  ["comp:ym-long", { year: "numeric", month: "long" }],
  ["comp:y-num", { year: "numeric" }],
  ["comp:m-long", { month: "long" }],
  ["comp:wd-long", { weekday: "long" }],
  ["comp:hm", { hour: "numeric", minute: "2-digit" }],
  ["comp:hms", { hour: "numeric", minute: "2-digit", second: "2-digit" }],
  ["comp:hm-12", { hour: "numeric", minute: "2-digit", hour12: true }],
  ["comp:hm-24", { hour: "numeric", minute: "2-digit", hour12: false }],
  ["comp:hms-tzshort", { hour: "numeric", minute: "2-digit", second: "2-digit", timeZoneName: "short" }],
  ["comp:hms-tzlong", { hour: "numeric", minute: "2-digit", second: "2-digit", timeZoneName: "long" }],
  ["comp:era", { era: "short", year: "numeric", month: "short", day: "numeric" }],
  ["comp:wymdhm-long", { weekday: "long", year: "numeric", month: "long", day: "numeric", hour: "numeric", minute: "2-digit" }],
  ["comp:default", {}],
];
for (const [tag, o] of comp) {
  optionSets.push({ tag, opts: o });
}

// Flexible day period (the dayPeriod option + B/b pattern fields): alone and
// combined with an hour, across all three widths. These exercise the
// dayPeriodRules range selection (morning1/afternoon1/evening1/night1/noon).
for (const dp of ["long", "short", "narrow"]) {
  optionSets.push({ tag: "dp:alone-" + dp, opts: { dayPeriod: dp } });
  optionSets.push({ tag: "dp:hour-" + dp, opts: { hour: "numeric", dayPeriod: dp } });
  optionSets.push({ tag: "dp:hm-" + dp, opts: { hour: "numeric", minute: "2-digit", dayPeriod: dp } });
}

// Named non-UTC time zones. Each (zone x timeZoneName) is rendered with
// hour:'numeric',minute:'2-digit' so the zone name is appended. The date set
// includes both summer and winter instants, so DST-observing zones exercise
// the daylight vs standard name selection. timeZone is carried in the saved
// opts (see below) so the Go harness applies it.
const zoneList = [
  "America/New_York",
  "Europe/London",
  "Asia/Kolkata",
  "Australia/Sydney",
  "Asia/Shanghai",
];
const zoneNames = ["short", "long", "shortGeneric", "longGeneric", "shortOffset", "longOffset"];
for (const tz of zoneList) {
  for (const zn of zoneNames) {
    optionSets.push({
      tag: "zone:" + tz + "/" + zn,
      opts: { hour: "numeric", minute: "2-digit", timeZone: tz, timeZoneName: zn },
    });
  }
}

const out = [];
for (const loc of locales) {
  for (const d of dates) {
    for (const os of optionSets) {
      const opts = Object.assign({ timeZone: "UTC" }, os.opts);
      let value;
      try {
        value = new Intl.DateTimeFormat(loc, opts).format(new Date(d));
      } catch (e) {
        continue;
      }
      out.push({
        locale: loc,
        ms: d,
        tag: os.tag,
        // Store the MERGED opts (incl. timeZone) so the Go harness applies the
        // same zone we formatted under. Existing cases keep timeZone:'UTC'.
        opts,
        value,
      });
    }
  }
}

process.stdout.write(JSON.stringify(out, null, 0));
