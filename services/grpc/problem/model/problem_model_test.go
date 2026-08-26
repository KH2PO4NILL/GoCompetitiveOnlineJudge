package model

import (
	"errors"
	"reflect"
	"testing"
)

func TestCanonicalTagsNormalizesAndDeduplicates(t *testing.T) {
	tags, err := canonicalTags([]string{" Graph ", "graph", "DP"})
	if err != nil {
		t.Fatalf("canonicalTags() error = %v", err)
	}
	if want := []string{"DP", "Graph"}; !reflect.DeepEqual(tags, want) {
		t.Fatalf("canonicalTags() = %#v, want %#v", tags, want)
	}
}

func TestValidateEditableRejectsInvalidDomainValues(t *testing.T) {
	valid := testEditableProblem()
	tests := []struct {
		name   string
		mutate func(*EditableProblem)
	}{
		{name: "empty title", mutate: func(value *EditableProblem) { value.Title = " " }},
		{name: "unspecified difficulty", mutate: func(value *EditableProblem) { value.Difficulty = 0 }},
		{name: "zero time limit", mutate: func(value *EditableProblem) { value.TimeLimitMS = 0 }},
		{name: "empty languages", mutate: func(value *EditableProblem) { value.AllowedLanguages = nil }},
		{name: "invalid visibility", mutate: func(value *EditableProblem) { value.Visibility = 0 }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			value := valid
			test.mutate(&value)
			if err := validateEditable(&value); !errors.Is(err, ErrInvalidProblemInput) {
				t.Fatalf("validateEditable() error = %v", err)
			}
		})
	}
}

func TestNormalizePageCapsPageSize(t *testing.T) {
	page, size := normalizePage(-1, 1000)
	if page != 1 || size != 100 {
		t.Fatalf("normalizePage() = (%d, %d), want (1, 100)", page, size)
	}
}
