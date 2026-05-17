package prism

type DiagnoseRequest struct {
	SessionID    string `json:"sessionId"`
	Explanation  string `json:"explanation" binding:"required"`
	FollowUpMode string `json:"followUpMode"`
}

type Diagnosis struct {
	CorrectParts   []string `json:"correctParts"`
	MissingParts   []string `json:"missingParts"`
	Misconceptions []string `json:"misconceptions"`
	Clarity        string   `json:"clarity"`
}

type FollowUp struct {
	Question string   `json:"question"`
	Reason   string   `json:"reason"`
	Targets  []string `json:"targets"`
}

type CoachResult struct {
	Diagnosis    Diagnosis `json:"diagnosis"`
	FollowUp     FollowUp  `json:"followUp"`
	Correction   string    `json:"correction"`
	MasteryScore int       `json:"masteryScore"`
	Intent       string    `json:"intent"`
}
