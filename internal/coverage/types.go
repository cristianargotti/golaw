package coverage

const (
	defaultFloor         = 80
	defaultMinStatements = 20
)

// Floors keeps thresholds independent so file exemptions cannot weaken wider scopes.
type Floors struct {
	Total         float64
	Package       float64
	File          float64
	MinStatements int
	Exemptions    []string
}

// DefaultFloors applies the kit's coverage law when no adopter policy overrides it.
func DefaultFloors() Floors {
	return Floors{
		Total: defaultFloor, Package: defaultFloor, File: defaultFloor,
		MinStatements: defaultMinStatements,
	}
}

// Measure retains integer counts so rounding cannot change a floor decision.
type Measure struct {
	Name       string
	Statements int
	Covered    int
}

// Percent treats an empty measured universe as covered because it has no missed statements.
func (m Measure) Percent() float64 {
	if m.Statements == 0 {
		return 100
	}
	return float64(m.Covered) * 100 / float64(m.Statements)
}

// Report separates failures from small measured units so missing sources cannot pass unnoticed.
type Report struct { //nolint:recvcheck // The public Lines method reads a value while evaluation helpers mutate the report.
	Total    Measure
	Packages []Measure
	Files    []Measure
	Failures []Measure
	Skipped  []Measure
}
