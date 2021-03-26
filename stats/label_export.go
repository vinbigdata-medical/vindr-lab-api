package stats

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type LabelExport struct {
	ID        string   `json:"id"`
	Created   int64    `json:"created"`
	FilePath  string   `json:"file_path"`
	CreatorID string   `json:"creator_id"`
	ProjectID string   `json:"project_id"`
	Tag       string   `json:"tag"`
	LabelIDs  []string `json:"label_ids"`
	Status    string   `json:"status"`
}

func (labelExport *LabelExport) String() string {
	b, _ := json.Marshal(labelExport)
	return string(b)
}

func (labelExport *LabelExport) New() {
	labelExport.ID = uuid.New().String()
	labelExport.Created = time.Now().UnixNano() / int64(time.Millisecond)
	labelExport.FilePath = labelExport.Tag + ".json"
}
