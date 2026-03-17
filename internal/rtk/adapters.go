package rtk

import (
	"encoding/json"
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
	MaxRounds *int
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
