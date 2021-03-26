package project

import (
	"encoding/json"

	"vindr-lab-api/constants"
)

var mapWorkflow = map[string]bool{
	constants.ProjWorkflowSingle:   true,
	constants.ProjWorkflowTriangle: true,
}

var mapProjectRole = map[string]bool{
	constants.ProjRoleAnnotator:    true,
	constants.ProjRoleProjectOwner: true,
	constants.ProjRoleReviewer:     true,
}

var mapLabelingType = map[string]bool{
	"3D": true,
	"2D": true,
}

type ProjectPerson struct {
	ID       string   `json:"id"`
	Username string   `json:"username"`
	Roles    []string `json:"roles"`
}

type Project struct {
	ID            string                 `json:"id"`
	Name          string                 `json:"name"`
	Description   string                 `json:"description"`
	CreatorID     string                 `json:"creator_id"`
	Created       int64                  `json:"created"`
	LabelGroupIDs []string               `json:"label_group_ids,omitempty"`
	People        []ProjectPerson        `json:"people,omitempty"`
	Workflow      string                 `json:"workflow"`
	DocumentLink  string                 `json:"document_link"`
	Meta          map[string]interface{} `json:"meta,omitempty"`
	RolesMapping  *map[string][]string   `json:"roles_mapping,omitempty"`
	Key           string                 `json:"key"`
	LabelingType  string                 `json:"labeling_type"`
}

func (project *Project) String() string {
	b, _ := json.Marshal(project)
	return string(b)
}

func (project *Project) IsValidWorkflow() bool {
	_, found := mapWorkflow[project.Workflow]
	return found
}

func IsValidUserRole(role string) bool {
	_, found := mapProjectRole[role]
	return found
}

func (project *Project) IsValidProjectRole() bool {
	for _, person := range project.People {
		for _, role := range person.Roles {
			if !IsValidUserRole(role) {
				return false
			}
		}
	}
	return true
}

func (project *Project) IsValidProject() bool {
	if project.CreatorID == "" || project.Name == "" || project.LabelingType == "" ||
		!project.IsValidProjectRole() || !project.IsValidWorkflow() {
		return false
	}
	return true
}

func (project *Project) retrieveRolesMapFromPeople() {
	people := project.People
	rolesMap := make(map[string][]string)
	for _, person := range people {
		for _, role := range person.Roles {
			rolesMap[role] = append(rolesMap[role], person.ID)
		}
	}
	project.RolesMapping = &rolesMap
}
