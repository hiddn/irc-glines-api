package abuse_glines

import (
	"net/http"
	"sync"
	"time"

	"github.com/labstack/echo/v4"
)

type getTasksStatus_api struct {
	UUID string `param:"uuid"`
}

type TaskStruct struct {
	TaskID        string      `json:"task_id"`
	UUID          string      `json:"uuid"`
	TaskType      string      `json:"task_type"`
	Progress      int64       `json:"progress"`
	CreationTS    int64       `json:"creation_ts"`
	StartedTS     int64       `json:"started_ts"`
	LastUpdatedTS int64       `json:"last_updated_ts"`
	CompletedTS   int64       `json:"completed_ts"`
	Message       string      `json:"message"`
	Data          interface{} `json:"-"`
	t             *TasksData  `json:"-"`
}

type TasksData struct {
	mu              sync.Mutex
	TasksMap        map[string][]TaskStruct
	CleanUpInterval int64
	ExpirationTime  int64
	stopChan        chan bool
}

func Tasks_init(expirationTime int64) *TasksData {
	t := &TasksData{
		TasksMap:        make(map[string][]TaskStruct),
		CleanUpInterval: 300,
		ExpirationTime:  expirationTime,
		stopChan:        make(chan bool),
	}
	go func() {
		ticker := time.NewTicker(time.Duration(t.CleanUpInterval) * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				t.CleanTasks()
			case <-t.stopChan:
				return
			}
		}
	}()
	return t
}

func (t *TasksData) Stop() {
	t.stopChan <- true
	t.mu.Lock()
	defer t.mu.Unlock()
	for k := range t.TasksMap {
		delete(t.TasksMap, k)
	}
}

func (t *TasksData) AddTask(uuid string, taskType string) *TaskStruct {
	task := &TaskStruct{
		UUID:        uuid,
		TaskID:      RandStringBytesRmndr(16),
		TaskType:    taskType,
		Progress:    0,
		CreationTS:  time.Now().Unix(),
		StartedTS:   0,
		CompletedTS: 0,
		Message:     "",
		Data:        nil,
		t:           t,
	}
	t.TasksMap[uuid] = append(t.TasksMap[taskType], *task)
	return task
}

func (task *TaskStruct) Start() {
	task.CreationTS = time.Now().Unix()
}

func (task *TaskStruct) SetProgress(progress int64) {
	task.Progress = progress
	task.LastUpdatedTS = time.Now().Unix()
}

func (task *TaskStruct) SetMessage(message string) {
	task.Message = message
	task.LastUpdatedTS = time.Now().Unix()
}

func (task *TaskStruct) SetData(data interface{}) {
	task.Data = data
	task.LastUpdatedTS = time.Now().Unix()
}

func (task *TaskStruct) Done() {
	task.CompletedTS = time.Now().Unix()
	task.Progress = 100
	task.LastUpdatedTS = time.Now().Unix()
}

func (task *TaskStruct) Cancel() {
	task.CompletedTS = time.Now().Unix()
	task.LastUpdatedTS = time.Now().Unix()
}

func (task *TaskStruct) IsExpired() bool {
	return task.CompletedTS > 0 && time.Now().Unix()-task.CompletedTS > task.t.ExpirationTime
}

func (t *TasksData) CleanTasks() {
	t.mu.Lock()
	defer t.mu.Unlock()
	for k, v := range t.TasksMap {
		for i, task := range v {
			if task.IsExpired() {
				t.TasksMap[k] = append(v[:i], v[i+1:]...)
			}
		}
		if len(t.TasksMap[k]) == 0 {
			delete(t.TasksMap, k)
		}
	}
}

func (t *TasksData) GetTasks(uuid string) []TaskStruct {
	return t.TasksMap[uuid]
}

func (t *TasksData) GetTasksStatus_api(c echo.Context) error {
	var api getTasksStatus_api
	if err := c.Bind(&api); err != nil {
		return c.String(http.StatusBadRequest, "Invalid request")
	}
	tasks := t.GetTasks(api.UUID)
	return c.JSON(http.StatusOK, tasks)
}
