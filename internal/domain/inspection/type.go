package inspection

// Type is the kind of inspection performed - a routine check versus a
// focused queen, brood, health, feeding, or seasonal inspection. It is a
// closed set: typeLabels below is the single source of truth for both
// which values are valid and what to call them, so adding a new
// inspection type means adding one entry there and nowhere else in the
// create/edit flow.
type Type string

// Values are UPPER_SNAKE_CASE, this project's convention for enum wire
// values (as opposed to Label, which is free-form display text).
const (
	TypeRoutine  Type = "ROUTINE"
	TypeQueen    Type = "QUEEN"
	TypeBrood    Type = "BROOD"
	TypeHealth   Type = "HEALTH"
	TypeFeeding  Type = "FEEDING"
	TypeSeasonal Type = "SEASONAL"
)

// typeLabels maps every valid Type to its human-readable label. Iteration
// order of a map isn't stable, so Types (below) exists separately for
// callers that need a stable, presentable ordering (e.g. populating a
// selection UI).
var typeLabels = map[Type]string{
	TypeRoutine:  "Routine",
	TypeQueen:    "Queen",
	TypeBrood:    "Brood",
	TypeHealth:   "Health",
	TypeFeeding:  "Feeding",
	TypeSeasonal: "Seasonal",
}

// Types lists every valid Type, in the order clients should present them.
var Types = []Type{TypeRoutine, TypeQueen, TypeBrood, TypeHealth, TypeFeeding, TypeSeasonal}

// Valid reports whether t is one of the known inspection types.
func (t Type) Valid() bool {
	_, ok := typeLabels[t]
	return ok
}

// Label returns t's human-readable label, or "" if t is not Valid.
func (t Type) Label() string {
	return typeLabels[t]
}
