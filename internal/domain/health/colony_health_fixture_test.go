package health

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/sbezhuk/beebase-inspection-service/internal/domain/inspection"
)

type fixtureRoot struct {
	Contract  string            `json:"contract"`
	AsOf      time.Time         `json:"asOf"`
	Scenarios []fixtureScenario `json:"scenarios"`
}
type fixtureScenario struct {
	Name        string              `json:"name"`
	Inspections []fixtureInspection `json:"inspections"`
	Expected    fixtureExpected     `json:"expected"`
}
type fixtureInspection struct {
	Type       string                 `json:"type"`
	Date       time.Time              `json:"date"`
	Assessment map[string]interface{} `json:"assessment"`
}
type fixtureExpected struct {
	State      string              `json:"state"`
	Coverage   string              `json:"coverage"`
	Dimensions map[string][]string `json:"dimensions"`
}

func TestCanonicalColonyHealthSemanticsV1(t *testing.T) {
	path := filepath.Join("..", "..", "..", "testdata", "colony_health", "v1", "colony_health_semantics_v1.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var root fixtureRoot
	if err := json.Unmarshal(data, &root); err != nil {
		t.Fatal(err)
	}
	if root.Contract != "colony_health_semantics_v1" {
		t.Fatalf("contract = %q", root.Contract)
	}
	policy := DefaultRecencyPolicyV1()
	for _, scenario := range root.Scenarios {
		t.Run(scenario.Name, func(t *testing.T) {
			var evidence []HealthEvidence
			for i, raw := range scenario.Inspections {
				in := inspection.Inspection{ID: uuid.NewSHA1(uuid.Nil, []byte{byte(i + 1)}), HiveID: uuid.Nil, InspectedAt: raw.Date, Type: fixtureType(raw.Type), Assessment: fixtureAssessment(raw.Assessment)}
				normalized := NormalizeInspection(in)
				evidence = append(evidence, normalized.HealthEvidence...)
			}
			dimensions, err := EvaluateDimensions(DimensionEvaluationInput{Evidence: evidence, AsOf: root.AsOf, RecencyPolicy: policy})
			if err != nil {
				t.Fatal(err)
			}
			result, err := EvaluateColonyHealth(dimensions)
			if err != nil {
				t.Fatal(err)
			}
			if string(result.State) != scenario.Expected.State || string(result.Coverage) != scenario.Expected.Coverage {
				t.Fatalf("aggregate = %s/%s, want %s/%s", result.State, result.Coverage, scenario.Expected.State, scenario.Expected.Coverage)
			}
			for _, dimension := range dimensions {
				want := scenario.Expected.Dimensions[string(dimension.Dimension)]
				if len(want) != 2 || string(dimension.State) != want[0] || string(dimension.Coverage) != want[1] {
					t.Fatalf("%s = %s/%s, want %v", dimension.Dimension, dimension.State, dimension.Coverage, want)
				}
			}
		})
	}
}

func fixtureType(value string) inspection.Type { return inspection.Type(value) }
func fixtureString(m map[string]interface{}, key string) *string {
	v, ok := m[key].(string)
	if !ok {
		return nil
	}
	return &v
}
func fixtureAssessment(m map[string]interface{}) *inspection.Assessment {
	if m == nil {
		return nil
	}
	a := &inspection.Assessment{Version: 1}
	if v := fixtureString(m, "colonyStrength"); v != nil {
		x := inspection.ColonyStrength(*v)
		a.ColonyStrength = &x
	}
	if v := fixtureString(m, "queenStatus"); v != nil {
		x := inspection.QueenStatus(*v)
		a.QueenStatus = &x
	}
	if v := fixtureString(m, "broodStatus"); v != nil {
		x := inspection.BroodStatus(*v)
		a.BroodStatus = &x
	}
	if v := fixtureString(m, "foodStores"); v != nil {
		x := inspection.FoodStores(*v)
		a.FoodStores = &x
	}
	if v := fixtureString(m, "healthConcerns"); v != nil {
		x := inspection.HealthConcerns(*v)
		a.HealthConcerns = &x
	}
	if v := fixtureString(m, "queenCondition"); v != nil {
		x := inspection.QueenCondition(*v)
		a.QueenCondition = &x
	}
	if v := fixtureString(m, "broodPattern"); v != nil {
		x := inspection.BroodPattern(*v)
		a.BroodPattern = &x
	}
	if v := fixtureString(m, "feedingNeed"); v != nil {
		x := inspection.FeedingNeed(*v)
		a.FeedingNeed = &x
	}
	if v := fixtureString(m, "feedingPerformed"); v != nil {
		x := inspection.FeedingPerformed(*v)
		a.FeedingPerformed = &x
	}
	if values, ok := m["pestSigns"].([]interface{}); ok {
		x := make([]inspection.PestSign, len(values))
		for i, value := range values {
			x[i] = inspection.PestSign(value.(string))
		}
		a.PestSigns = &x
	}
	if values, ok := m["healthWarningSigns"].([]interface{}); ok {
		x := make([]inspection.HealthWarningSign, len(values))
		for i, value := range values {
			x[i] = inspection.HealthWarningSign(value.(string))
		}
		a.HealthWarningSigns = &x
	}
	if values, ok := m["broodStages"].([]interface{}); ok {
		x := make([]inspection.BroodStage, len(values))
		for i, value := range values {
			x[i] = inspection.BroodStage(value.(string))
		}
		a.BroodStages = &x
	}
	if values, ok := m["feedTypes"].([]interface{}); ok {
		x := make([]inspection.FeedType, len(values))
		for i, value := range values {
			x[i] = inspection.FeedType(value.(string))
		}
		a.FeedTypes = &x
	}
	return a
}
