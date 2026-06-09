package datetime

import (
	"strings"
	"time"
)

// metazonePeriod is one entry of the global zone->metazone mapping
// (CLDR metaZones.json metazoneInfo). A zone may map to different metazones
// over time; From/To bound the interval in which Mzone applies (Unix epoch
// seconds, 0 meaning open-ended). The generator emits the package-level
// zoneToMetazone map of these.
type metazonePeriod struct {
	Mzone string
	From  int64 // unix seconds, 0 = open start
	To    int64 // unix seconds, 0 = open end
}

// zoneToMetazone maps a CLDR (legacy) zone id to its ordered metazone periods.
// It is declared here but POPULATED by the generated tables_gen.go (via an
// init that assigns the full map), so the package still builds against an older
// table that predates the metazone data (the map is simply nil/empty then).
var zoneToMetazone map[string][]metazonePeriod

// zoneToTerritory maps a CLDR (legacy) zone id to its territory code (from the
// BCP-47 time-zone alias table). zoneUsesCountry is the subset of zones whose
// generic-location name uses the COUNTRY (territory) display name rather than
// the exemplar city (single-zone territory or the territory's primaryZone).
// Both are declared here and POPULATED by the generated tables_gen.go.
var (
	zoneToTerritory map[string]string
	zoneUsesCountry map[string]bool
)

// metazoneFor returns the CLDR metazone id for a zone id at instant t, picking
// the period whose [From,To) interval covers t (open-ended bounds match).
func metazoneFor(zoneID string, t time.Time) string {
	periods := zoneToMetazone[zoneID]
	if len(periods) == 0 {
		return ""
	}
	if len(periods) == 1 {
		return periods[0].Mzone
	}
	sec := t.Unix()
	for _, p := range periods {
		if (p.From == 0 || sec >= p.From) && (p.To == 0 || sec < p.To) {
			return p.Mzone
		}
	}
	// Default to the last (most recent) period for modern times.
	return periods[len(periods)-1].Mzone
}

// zoneObservesDST reports whether the zone of t observes daylight saving at any
// point in the surrounding year. ICU resolves a long generic zone name to the
// zone's standard name when the zone never observes DST (no ambiguity), so we
// need this distinct from the instant's own IsDST().
func zoneObservesDST(t time.Time) bool {
	loc := t.Location()
	year := t.Year()
	for month := 1; month <= 12; month++ {
		probe := time.Date(year, time.Month(month), 15, 12, 0, 0, 0, loc)
		if probe.IsDST() {
			return true
		}
	}
	return false
}

// specificZoneName renders the z field (specific non-location name): the
// standard or daylight metazone name selected by the instant's DST state,
// honoring per-zone overrides. Returns "" when no name is available (caller
// falls back to the localized GMT offset).
func (c *formatCtx) specificZoneName(t time.Time, long bool) string {
	if c.zoneID == "" {
		return ""
	}
	width := "short"
	if long {
		width = "long"
	}
	typ := "standard"
	if t.IsDST() {
		typ = "daylight"
	}
	// Per-zone override first (e.g. Europe/London long.daylight =
	// "British Summer Time").
	if v := c.zoneNameOverride(width, typ); v != "" {
		return v
	}
	mz := metazoneFor(c.zoneID, t)
	if mz == "" {
		return ""
	}
	names := c.ld.MetazoneNames[mz]
	if names == nil {
		return ""
	}
	return names[width+"."+typ]
}

// genericZoneName renders the v/V field (generic non-location name) following
// ICU's TimeZoneFormat fallback order for a generic name at the requested width
// W:
//
//  1. per-zone override generic[W]
//     2a. (zone observes DST)   metazone[W].generic
//     2b. (zone never observes DST) metazone[W].standard — ICU uses the STANDARD
//     name as the generic for a zone with no DST ambiguity, even when CLDR
//     also provides a [W].generic (e.g. China longGeneric resolves to
//     "China Standard Time", not the metazone generic "China Time"; hi
//     Kolkata shortGeneric resolves to "IST", the metazone short.standard).
//  3. generic LOCATION format (regionFormat with a country or city name)
//  4. "" -> caller uses the localized GMT offset
//
// The width is NOT cross-fallen-back (short never borrows the long name): a
// zone with no metazone name at the requested width goes straight to the
// location format (e.g. Sydney shortGeneric = "Sydney Time", not the long
// generic "Eastern Australia Time"; en Kolkata shortGeneric = "India Time",
// the country location, because the India metazone has no short block at all).
func (c *formatCtx) genericZoneName(t time.Time, long bool) string {
	if c.zoneID == "" {
		return ""
	}
	width := "short"
	if long {
		width = "long"
	}
	// 1. per-zone generic override.
	if v := c.zoneNameOverride(width, "generic"); v != "" {
		return v
	}
	// 2. metazone name at the requested width: generic for DST-observing zones,
	// standard for zones that never observe DST.
	typ := "generic"
	if !c.zoneObservesDST {
		typ = "standard"
	}
	mz := metazoneFor(c.zoneID, t)
	if names := c.ld.MetazoneNames[mz]; names != nil {
		if v := names[width+"."+typ]; v != "" {
			return v
		}
	}
	// 3. generic location format.
	if v := c.genericLocation(); v != "" {
		return v
	}
	// 4. fall back to the localized GMT offset (handled by the caller).
	return ""
}

// zoneNameOverride looks up a per-zone name override ("<width>.<type>").
func (c *formatCtx) zoneNameOverride(width, typ string) string {
	if c.zoneID == "" {
		return ""
	}
	if ov := c.ld.ZoneOverrides[c.zoneID]; ov != nil {
		return ov[width+"."+typ]
	}
	return ""
}

// genericLocation builds the regionFormat generic-location name. Following
// ICU's country-vs-city rule, it uses the COUNTRY (territory) display name when
// the zone is its territory's representative (single-zone territory or the
// territory's primaryZone, recorded in zoneUsesCountry), otherwise the exemplar
// CITY. The chosen name is substituted into the locale's regionFormat
// ("{0} Time", de "{0} (Ortszeit)", fr "heure : {0}"). Returns "" when neither
// a usable name nor the regionFormat is available (caller uses the GMT offset).
func (c *formatCtx) genericLocation() string {
	region := c.ld.Zones["regionFormat"]
	if region == "" {
		return ""
	}
	var name string
	if zoneUsesCountry[c.zoneID] {
		if terr := zoneToTerritory[c.zoneID]; terr != "" {
			name = c.ld.TerritoryNames[terr]
		}
	}
	if name == "" {
		name = c.exemplarCity()
	}
	if name == "" {
		return ""
	}
	return strings.Replace(region, "{0}", name, 1)
}

// exemplarCity returns the zone's localized exemplar city: the explicit CLDR
// exemplarCity when present, otherwise ICU's fallback derivation from the IANA
// zone id (last '/'-segment with underscores turned into spaces, e.g.
// "America/New_York" -> "New York"). The legacy CLDR id is used for the
// explicit lookup; the canonical id (c.canonicalZone) for the derivation.
func (c *formatCtx) exemplarCity() string {
	if v := c.ld.ExemplarCities[c.zoneID]; v != "" {
		return v
	}
	id := c.canonicalZone
	if id == "" {
		id = c.zoneID
	}
	if i := strings.LastIndexByte(id, '/'); i >= 0 {
		id = id[i+1:]
	}
	return strings.ReplaceAll(id, "_", " ")
}
