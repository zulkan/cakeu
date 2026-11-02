package service

import (
	"strings"
	"testing"

	"goskeleton/internal/spreadsheet"
)

func TestSampleRowNumbersStaysWithinBounds(t *testing.T) {
	got := sampleRowNumbers(100, 30)
	if len(got) != 30 {
		t.Fatalf("expected 30 rows, got %d", len(got))
	}
	for _, row := range got {
		if row < 1 || row > 100 {
			t.Fatalf("row %d out of bounds", row)
		}
	}
	if got[0] != 1 || got[len(got)-1] != 100 {
		t.Fatalf("expected first and last rows, got %v", got)
	}
}

func TestColumnLetter(t *testing.T) {
	cases := map[int]string{1: "A", 26: "Z", 27: "AA", 52: "AZ", 53: "BA"}
	for input, want := range cases {
		if got := columnLetter(input); got != want {
			t.Fatalf("columnLetter(%d) = %q, want %q", input, got, want)
		}
	}
}

func TestBuildImportMappingRequestTruncatesCells(t *testing.T) {
	table := &spreadsheet.Table{
		Name: "JANUARI",
		Rows: [][]string{{"header"}, {strings.Repeat("x", 200)}},
	}
	request := buildImportMappingRequest("budgeting 2026.xlsx", table, nil, nil, 5)

	if request.Filename != "budgeting 2026.xlsx" || request.Sheet != "JANUARI" {
		t.Fatalf("unexpected request metadata: %+v", request)
	}
	if len(request.Sample) == 0 || len(request.Sample[0].Cells) == 0 {
		t.Fatalf("expected a sample")
	}
	for _, sample := range request.Sample {
		for _, value := range sample.Cells {
			if len(value) > 100 {
				t.Fatalf("cell not truncated: %d chars", len(value))
			}
		}
	}
}

func TestImportMappingMessagesStateJSONOnly(t *testing.T) {
	request := importMappingRequest{Sheet: "JANUARI"}
	messages := importMappingMessages(request)
	if len(messages) != 2 || messages[0].Role != "system" || messages[1].Role != "user" {
		t.Fatalf("unexpected messages: %+v", messages)
	}
	if !strings.Contains(strings.ToLower(messages[0].Content), "json") {
		t.Fatalf("system prompt must require JSON")
	}
	if !strings.Contains(messages[1].Content, "JANUARI") {
		t.Fatalf("user payload must include the sheet name")
	}
}

func TestParseImportMappingResponseStripsFences(t *testing.T) {
	reply := "```json\n{\"header_row\":7,\"fields\":{\"date\":{\"column\":5}},\"type\":{\"constant\":\"expense\"}}\n```"
	plan, err := parseImportMappingResponse(reply)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if plan.HeaderRow != 7 || plan.Fields.Date.Column != 5 || plan.Type.Constant != "expense" {
		t.Fatalf("unexpected plan: %+v", plan)
	}
}

func TestParseImportMappingResponseRejectsNoise(t *testing.T) {
	if _, err := parseImportMappingResponse("I cannot do that"); err == nil {
		t.Fatal("expected an error")
	}
}

func TestSampleRowNumbersDistributesAcrossSheet(t *testing.T) {
	got := sampleRowNumbers(100, 30)
	if len(got) != 30 {
		t.Fatalf("expected 30 rows, got %d", len(got))
	}
	for _, row := range got {
		if row < 1 || row > 100 {
			t.Fatalf("row %d out of bounds", row)
		}
	}

	// Middle row must be present so the model sees representative content
	found50 := false
	for _, row := range got {
		if row == 50 {
			found50 = true
			break
		}
	}
	if !found50 {
		t.Fatalf("expected row 50 to be sampled, got %v", got)
	}

	// Largest gap between consecutive rows after first block should be ≤ 8
	// First block is rows 1..12, remainder starts after that
	var maxGap int
	for i := 1; i < len(got); i++ {
		gap := got[i] - got[i-1]
		if gap > maxGap {
			maxGap = gap
		}
	}
	if maxGap > 8 {
		t.Fatalf("largest gap between sampled rows is %d (> 8): %v", maxGap, got)
	}
}
