package rtk

type Proposal struct {
	Summary    string   `json:"summary"`
	Claims     []string `json:"claims"`
	Acceptance []string `json:"acceptance"`
}

type Evidence struct {
	ID          string `json:"id"`
	Source      string `json:"source"`
	Kind        string `json:"kind"`
	Phase       string `json:"phase"`
	Statement   string `json:"statement"`
	Excerpt     string `json:"excerpt"`
	CollectedBy string `json:"collected_by"`
	Round       int    `json:"round"`
	CreatedAt   string `json:"created_at"`
}

type Finding struct {
	ID              string   `json:"id"`
	Critic          string   `json:"critic"`
	Severity        string   `json:"severity"`
	Basis           string   `json:"basis"`
	Summary         string   `json:"summary"`
	Rationale       string   `json:"rationale"`
	SuggestedChange string   `json:"suggested_change"`
	EvidenceIDs     []string `json:"evidence_ids"`
}

type Decision struct {
	FindingID   string   `json:"finding_id"`
	Disposition string   `json:"disposition"`
	Rationale   string   `json:"rationale"`
	EvidenceIDs []string `json:"evidence_ids"`
}

type Verdict struct {
	Summary         string     `json:"summary"`
	RevisedProposal *Proposal  `json:"revised_proposal"`
	Decisions       []Decision `json:"decisions"`
}

type SessionError struct {
	Message string  `json:"message"`
	Actor   *string `json:"actor,omitempty"`
	Phase   *string `json:"phase,omitempty"`
	At      string  `json:"at"`
}

type Status struct {
	Round            int           `json:"round"`
	Converged        bool          `json:"converged"`
	UnresolvedHigh   int           `json:"unresolved_high"`
	UnresolvedMedium int           `json:"unresolved_medium"`
	State            string        `json:"state"`
	ActiveActor      *string       `json:"active_actor"`
	ActivePhase      *string       `json:"active_phase"`
	Error            *SessionError `json:"error"`
}

type Counts struct {
	Total    int `json:"total"`
	High     int `json:"high"`
	Medium   int `json:"medium"`
	Low      int `json:"low"`
	Material int `json:"material"`
	Gaps     int `json:"gaps"`
}

type PhaseRecord struct {
	Actor         string         `json:"actor"`
	Phase         string         `json:"phase"`
	Status        string         `json:"status"`
	InputSummary  map[string]any `json:"input_summary"`
	OutputSummary map[string]any `json:"output_summary"`
	Artifact      map[string]any `json:"artifact"`
	StartedAt     string         `json:"started_at"`
	CompletedAt   *string        `json:"completed_at"`
	DurationMS    *int64         `json:"duration_ms"`
	Error         *SessionError  `json:"error"`
}

type RoundRecord struct {
	Index                   int           `json:"index"`
	Proposal                *Proposal     `json:"proposal"`
	EvidenceAdded           []string      `json:"evidence_added"`
	FindingsAgainstProposal []Finding     `json:"findings_against_proposal"`
	FindingsLegacy          []Finding     `json:"findings,omitempty"`
	ReviewSummary           Counts        `json:"review_summary"`
	Verdict                 *Verdict      `json:"verdict"`
	PhaseHistory            []PhaseRecord `json:"phase_history"`
	CreatedAt               string        `json:"created_at"`
	UpdatedAt               string        `json:"updated_at"`
	Error                   *SessionError `json:"error"`
}

func (r *RoundRecord) CurrentFindings() []Finding {
	if r == nil {
		return nil
	}
	if len(r.FindingsAgainstProposal) > 0 {
		return r.FindingsAgainstProposal
	}
	return r.FindingsLegacy
}

type Session struct {
	Version             int           `json:"version"`
	ID                  string        `json:"id"`
	Topic               string        `json:"topic"`
	CreatedAt           string        `json:"created_at"`
	Chair               string        `json:"chair"`
	Critics             []string      `json:"critics"`
	MaxRounds           *int          `json:"max_rounds"`
	Adapter             string        `json:"adapter"`
	Evidence            []Evidence    `json:"evidence"`
	Rounds              []RoundRecord `json:"rounds"`
	OpenRound           *RoundRecord  `json:"open_round"`
	AdjudicatedProposal *Proposal     `json:"adjudicated_proposal"`
	Status              Status        `json:"status"`
}

type SessionSummary struct {
	ID                    string                    `json:"id"`
	Topic                 string                    `json:"topic"`
	Chair                 string                    `json:"chair"`
	Critics               []string                  `json:"critics"`
	MaxRounds             *int                      `json:"max_rounds"`
	Round                 int                       `json:"round"`
	State                 string                    `json:"state"`
	Converged             bool                      `json:"converged"`
	UnresolvedHigh        int                       `json:"unresolved_high"`
	UnresolvedMedium      int                       `json:"unresolved_medium"`
	ActiveActor           *string                   `json:"active_actor"`
	ActivePhase           *string                   `json:"active_phase"`
	ErrorMessage          string                    `json:"error_message"`
	EvidenceCount         int                       `json:"evidence_count"`
	TotalFindings         int                       `json:"total_findings"`
	GapFindings           int                       `json:"gap_findings"`
	AdjudicatedSummary    string                    `json:"adjudicated_summary"`
	LatestProposalSummary string                    `json:"latest_proposal_summary"`
	FindingsByCritic      map[string]CountsByCritic `json:"findings_by_critic"`
	HasOpenRound          bool                      `json:"has_open_round"`
	UpdatedAt             string                    `json:"updated_at"`
	CreatedAt             string                    `json:"created_at"`
}

type CountsByCritic struct {
	Total  int `json:"total"`
	High   int `json:"high"`
	Medium int `json:"medium"`
	Low    int `json:"low"`
	Gap    int `json:"gap"`
}

func stringPtr(value string) *string {
	if value == "" {
		return nil
	}
	copy := value
	return &copy
}

func intPtr(value int) *int {
	copy := value
	return &copy
}

func int64Ptr(value int64) *int64 {
	copy := value
	return &copy
}
