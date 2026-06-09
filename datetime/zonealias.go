package datetime

// zoneCanonicalToCLDR bridges modern canonical IANA time-zone ids to the LEGACY
// IANA ids that CLDR keys its metazone / zone data under. Go's time package and
// ECMAScript's Intl use the canonical ("current") IANA names (e.g.
// "Asia/Kolkata"), whereas CLDR's metaZones.json and timeZoneNames.json still
// key the corresponding data under the older "backward"-compatibility names
// (e.g. "Asia/Calcutta"). This is the same renaming ICU applies internally when
// it canonicalizes a zone before looking up CLDR data.
//
// This map is intentionally HAND-WRITTEN (not generated): cldr-bcp47 — which
// carries the machine-readable IANA alias table — is not installed in the
// generation toolchain, so there is no generated source for it. It is kept
// focused on the common, well-known IANA "backward" renames. Any zone NOT in
// this map and not already a CLDR-canonical id simply falls back to the
// localized GMT offset, which is itself numerically correct.
var zoneCanonicalToCLDR = map[string]string{
	"Africa/Asmara":                  "Africa/Asmera",
	"America/Nuuk":                   "America/Godthab",
	"America/Argentina/Buenos_Aires": "America/Buenos_Aires",
	"America/Argentina/Catamarca":    "America/Catamarca",
	"America/Argentina/Cordoba":      "America/Cordoba",
	"America/Argentina/Jujuy":        "America/Jujuy",
	"America/Argentina/Mendoza":      "America/Mendoza",
	"America/Indiana/Indianapolis":   "America/Indianapolis",
	"America/Kentucky/Louisville":    "America/Louisville",
	"Asia/Ho_Chi_Minh":               "Asia/Saigon",
	"Asia/Kathmandu":                 "Asia/Katmandu",
	"Asia/Kolkata":                   "Asia/Calcutta",
	"Asia/Yangon":                    "Asia/Rangoon",
	"Atlantic/Faroe":                 "Atlantic/Faeroe",
	"Europe/Kyiv":                    "Europe/Kiev",
	"Pacific/Chuuk":                  "Pacific/Truk",
	"Pacific/Pohnpei":                "Pacific/Ponape",
	"America/Atikokan":               "America/Coral_Harbour",
	"Asia/Qostanay":                  "Asia/Qostanay",
	"Australia/Currie":               "Australia/Currie",
}

// cldrZoneID returns the CLDR (legacy) zone key for a canonical IANA id,
// translating via the alias map when needed. Ids not in the map are returned
// unchanged (most modern canonical ids — US/Europe/Tokyo/Shanghai/Sydney/etc.
// — already match CLDR's keys directly).
func cldrZoneID(iana string) string {
	if v, ok := zoneCanonicalToCLDR[iana]; ok {
		return v
	}
	return iana
}
