package label_group

import (
	"encoding/json"
)

type LabelGroup struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Color     string   `json:"color"`
	Created   int64    `json:"created"`
	CreatorID string   `json:"creator_id"`
	OwnerIDs  []string `json:"owner_ids,omitempty"`
}

func (labelGroup *LabelGroup) String() string {
	b, _ := json.Marshal(labelGroup)
	return string(b)
}
