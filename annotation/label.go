package annotation

import (
	"encoding/json"
	"vindr-lab-api/constants"
)

// LabelType enums of Label
var LabelType = map[string]bool{
	constants.LabelTypeImpression: true,
	constants.LabelTypeFinding:    true,
}

// LabelScope enums of Label
var LabelScope = map[string]bool{
	constants.LabelScopeStudy:  true,
	constants.LabelScopeSeries: true,
	constants.LabelScopeImage:  true,
}

var AnnotationType = map[string]bool{
	constants.AntnTypeTag:     true,
	constants.AntnTypeBox:     true,
	constants.AntnTypePolygon: true,
	constants.AntnTypeMask:    true,
	constants.AntnType3DBox:   true,
}

var LabelChildrenSelectType = map[string]bool{
	constants.LabelSelectTypeNone:     true,
	constants.LabelSelectTypeRadio:    true,
	constants.LabelSelectTypeCheckbox: true,
}

type Label struct {
	ID                 string  `json:"id"`
	Type               string  `json:"type"`
	Scope              string  `json:"scope"`
	AnnotationType     string  `json:"annotation_type"`
	Name               string  `json:"name"`
	ShortName          string  `json:"short_name"`
	ParentLabelID      string  `json:"parent_label_id"`
	Description        string  `json:"description,omitempty"`
	Color              string  `json:"color"`
	CreatorID          string  `json:"creator_id"`
	Created            int64   `json:"created,omitempty"`
	LabelGroupID       string  `json:"label_group_id,omitempty"`
	SubLabels          []Label `json:"sub_labels,omitempty"`
	ChildrenSelectType string  `json:"children_select_type"`
	Order              float32 `json:"order"`
}

func (label *Label) IsValidLabelScope() bool {
	_, found := LabelScope[label.Scope]
	return found
}

func (label *Label) IsValidAnnotationType() bool {
	_, found := AnnotationType[label.AnnotationType]
	return found
}

func (label *Label) IsValidLabelType() bool {
	_, found := LabelType[label.Type]
	return found
}

func (label *Label) IsValidLabelChildrenSelection() bool {
	_, found := LabelType[label.Type]
	return found
}

func (label *Label) IsValidLabel() bool {
	if label.CreatorID == "" || label.LabelGroupID == "" || label.Color == "" || label.Name == "" ||
		!label.IsValidAnnotationType() || !label.IsValidLabelScope() || !label.IsValidLabelType() ||
		!label.IsValidLabelChildrenSelection() {
		return false
	}
	return true
}

func (label *Label) String() string {
	b, _ := json.Marshal(label)
	return string(b)
}
