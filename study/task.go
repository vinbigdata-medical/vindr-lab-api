package study

import (
	"encoding/json"
	"fmt"
	"time"

	"vindr-lab-api/constants"

	"github.com/google/uuid"
)

var mapTaskStatus = map[string]int{
	constants.TaskStatusNew:       0,
	constants.TaskStatusDoing:     1,
	constants.TaskStatusCompleted: 2,
}

var mapTaskType = map[string]bool{
	constants.TaskTypeAnnotate: true,
	constants.TaskTypeReview:   true,
}

type TaskSubmit struct {
	ProjectID   string   `json:"project_id"`
	AssigneeIDs []string `json:"assignee_ids"`
	StudyIDs    []string `json:"study_ids"`
	Type        string   `json:"type"`
}

type TaskAssignment2 struct {
	ProjectID         string              `json:"project_id"`
	AssigneeIDs       map[string][]string `json:"assignee_ids"`
	StudyIDs          []string            `json:"study_ids"`
	Strategy          string              `json:"strategy"`
	SourceType        string              `json:"source_type"`
	StudyInstanceUIDs []string            `json:"study_instance_uids"`
	SearchQuery       struct {
		Size   int    `json:"size"`
		Query  string `json:"query"`
		Status string `json:"status"`
	} `json:"search_query"`
}

type Task struct {
	ID         string `json:"id"`
	Code       string `json:"code"`
	ProjectID  string `json:"project_id"`
	CreatorID  string `json:"creator_id"`
	AssigneeID string `json:"assignee_id"`
	StudyID    string `json:"study_id"`
	Status     string `json:"status"`
	Created    int64  `json:"created"`
	Modified   int64  `json:"modified"`
	Type       string `json:"type"`
	StudyCode  string `json:"study_code,omitempty"`
	Study      *Study `json:"study,omitempty"`
	Comment    string `json:"comment"`
	Archived   bool   `json:"archived"`
}

func (task *Task) NewTask(assgineeID, studyID, projectID, taskType string) {
	nowNanos := time.Now().UnixNano()
	now := nowNanos / int64(time.Millisecond)
	task.AssigneeID = assgineeID
	task.StudyID = studyID
	task.Created = now
	task.Modified = now
	task.ProjectID = projectID
	task.Type = taskType
	task.ID = uuid.New().String()
	task.Status = constants.TaskStatusNew
	task.Archived = false
}

func (task *TaskSubmit) String() string {
	b, _ := json.Marshal(task)
	return string(b)
}

func (taskItem *Task) String() string {
	b, _ := json.Marshal(taskItem)
	return string(b)
}

func IsValidTaskStatus(taskStatus string) bool {
	_, found := mapTaskStatus[taskStatus]
	return found
}

func IsValidTaskType(taskType string) bool {
	_, found := mapTaskType[taskType]
	return found
}

func (task *Task) IsValidTask() bool {
	if !IsValidTaskStatus(task.Status) || !IsValidTaskType(task.Type) {
		return false
	}
	return true
}

func (taskSubmit *TaskSubmit) IsValidTaskType() bool {
	_, found := mapTaskType[taskSubmit.Type]
	return found
}

func (taskSubmit *TaskSubmit) IsValidTaskSubmit() bool {
	if !taskSubmit.IsValidTaskType() ||
		len(taskSubmit.AssigneeIDs) == 0 ||
		len(taskSubmit.StudyIDs) == 0 ||
		taskSubmit.ProjectID == "" {
		return false
	}
	return true
}

func (taskAssignment2 *TaskAssignment2) IsValidData() bool {
	switch taskAssignment2.SourceType {
	case constants.ASSIGN_SOURCE_FILE:
		if len(taskAssignment2.StudyInstanceUIDs) > 0 {
			return true
		}
		break
	case constants.ASSIGN_SOURCE_SELECTED:
		if len(taskAssignment2.StudyIDs) > 0 {
			return true
		}
		break
	case constants.ASSIGN_SOURCE_SEARCH:
		if taskAssignment2.SearchQuery.Size > 0 || taskAssignment2.SearchQuery.Query != "" {
			return true
		}
		break
	}
	return false
}

func (ta *TaskAssignment2) IsValidStrategy() bool {
	switch ta.Strategy {
	case constants.ASSIGN_STRATEGY_ALL, constants.ASSIGN_STRATEGY_EQUALLY:
		return true
	default:
		return false
	}
}

//TODO: update validation for task assignment
func (taskAssignment2 *TaskAssignment2) IsValidTaskAssignment2() bool {
	fmt.Println(taskAssignment2.IsValidData(), taskAssignment2.IsValidStrategy())
	if !taskAssignment2.IsValidData() ||
		!taskAssignment2.IsValidStrategy() {
		return false
	}
	return true
}
