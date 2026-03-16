package rtk

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type EvidenceBatch struct {
	Items       []EvidenceInput `json:"items"`
	CollectedBy string          `json:"collected_by"`
	Phase       string          `json:"phase"`
}

type CollectEvidenceArgs struct {
	Session *Session
	Round   int
	Actor   string
	Phase   string
}

type ProposeArgs struct {
	Session *Session
	Round   int
}

type ReviewArgs struct {
	Session        *Session
	Round          int
	Critic         string
	Proposal       *Proposal
	EvidenceKeyMap map[string]string
}

type AdjudicateArgs struct {
	Session        *Session
	Round          int
	Proposal       *Proposal
	Findings       []Finding
	EvidenceKeyMap map[string]string
}

type AdapterMetadata struct {
	Topic     string
	Chair     string
	Critics   []string
	MaxRounds int
}

type Adapter interface {
	Kind() string
	Metadata() AdapterMetadata
	SeedEvidence(context.Context) ([]EvidenceBatch, error)
	CollectEvidence(context.Context, CollectEvidenceArgs) ([]EvidenceBatch, error)
	Propose(context.Context, ProposeArgs) (*Proposal, error)
	Review(context.Context, ReviewArgs) ([]Finding, error)
	Adjudicate(context.Context, AdjudicateArgs) (*Verdict, error)
}

type AdapterConfig struct {
	FixturePath   string
	SpecPath      string
	TelemetryFile string
}

func CreateAdapter(kind string, config AdapterConfig) (Adapter, error) {
	switch kind {
	case "fixture":
		return newFixtureAdapter(config.FixturePath)
	case "exec":
		return newExecAdapter(config.SpecPath, config.TelemetryFile)
	default:
		return nil, fmt.Errorf("unsupported adapter kind %s", kind)
	}
}

func readJSONFile(path string, target any) error {
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return err
	}
	return json.Unmarshal(data, target)
}

func durationMS(value int) time.Duration {
	return time.Duration(value) * time.Millisecond
}
