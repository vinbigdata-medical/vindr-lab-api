package study

import (
	"testing"
	"vindr-lab-api/constants"

	"github.com/stretchr/testify/assert"
)

var taskSubmit = TaskSubmit{
	ProjectID:   "project",
	AssigneeIDs: []string{"a1"},
	StudyIDs:    []string{"s1"},
	Type:        constants.TaskTypeAnnotate,
}

var task = Task{
	AssigneeID: "assginee",
	Code:       "code",
	Created:    0,
	CreatorID:  "creator",
	ID:         "id",
	Modified:   0,
	ProjectID:  "project",
	Status:     constants.TaskStatusCompleted,
	Study:      &Study{},
	StudyID:    "study",
	Type:       constants.TaskTypeAnnotate,
}

func TestNewTask(t *testing.T) {
	{
		task := Task{}
		// taskSubmit := TaskSubmit{}
		task.NewTask(taskSubmit.AssigneeIDs[0], taskSubmit.StudyIDs[0], taskSubmit.ProjectID, taskSubmit.Type)
		assert.Greater(t, task.Created, int64(0))
	}
}

func TestString(t *testing.T) {
	{
		assert.NotEqual(t, "{}", task.String())
	}
	{
		assert.NotEqual(t, "{}", taskSubmit.String())
	}
}

func TestIsValidTaskStatus(t *testing.T) {
	{
		assert.Equal(t, true, IsValidTaskStatus(task.Status))
	}
}

func TestIsValidTaskType(t *testing.T) {
	{
		assert.Equal(t, true, IsValidTaskType(task.Type))
	}
}

func TestIsValidTask(t *testing.T) {
	{
		task := task
		task.Status = "x"
		assert.Equal(t, false, task.IsValidTask())
	}
	{
		assert.Equal(t, true, task.IsValidTask())
	}
}

func TestIsValidTaskSubmit(t *testing.T) {
	{
		assert.Equal(t, true, taskSubmit.IsValidTaskSubmit())
	}
	{
		taskSubmit := taskSubmit
		taskSubmit.Type = "x"
		assert.Equal(t, false, taskSubmit.IsValidTaskSubmit())
	}
}
