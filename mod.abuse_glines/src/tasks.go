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
	mu                    sync.Mutex
	TaskID                string      `json:"task_id"`
	UUID                  string      `json:"uuid"`
	TaskType              string      `json:"task_type"`
	Progress              int64       `json:"progress"`
	CreationTS            int64       `json:"creation_ts"`
	StartedTS             int64       `json:"started_ts"`
	LastUpdatedTS         int64       `json:"last_updated_ts"`
	EndTS                 int64       `json:"end_ts"`
	Message               string      `json:"message"`
	DataVisibleToUser     string      `json:"data"` // email goes here when TaskType is confirmemail
	Data                  interface{} `json:"-"`
	T                     *TasksData  `json:"-"`
	LastViewedTS          int64       `json:"-"`
	ModifiedSinceLastView bool        `json:"-"`
}

type TasksData struct {
	mu              sync.Mutex
	TasksMap        map[string][]*TaskStruct
	CleanUpInterval int64
	ExpirationTime  int64
	stopChan        chan bool
}

func Tasks_init(expirationTime int64) *TasksData {
	t := &TasksData{
		TasksMap:        make(map[string][]*TaskStruct),
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
		UUID:       uuid,
		TaskID:     RandStringBytesRmndr(16),
		TaskType:   taskType,
		Progress:   0,
		CreationTS: time.Now().Unix(),
		StartedTS:  0,
		EndTS:      0,
		Message:    "",
		Data:       nil,
		T:          t,
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if _, ok := t.TasksMap[uuid]; !ok {
		t.TasksMap[uuid] = make([]*TaskStruct, 0, 8)
	}
	t.TasksMap[uuid] = append(t.TasksMap[uuid], task)
	return task
}

func (task *TaskStruct) MarkAsModified() {
	/* Do not lock the task here, as this function is called from within a lock */
	task.ModifiedSinceLastView = true
	task.LastUpdatedTS = time.Now().Unix()
}

func (task *TaskStruct) Start() {
	task.mu.Lock()
	defer task.mu.Unlock()
	task.StartedTS = time.Now().Unix()
	task.MarkAsModified()
}

func (task *TaskStruct) SetProgress(progress int64, message string) {
	task.mu.Lock()
	defer task.mu.Unlock()
	task.Progress = progress
	if task.Message != "" {
		task.Message = message
	}
	task.MarkAsModified()
}

func (task *TaskStruct) SetMessage(message string) {
	task.mu.Lock()
	defer task.mu.Unlock()
	task.Message = message
	task.MarkAsModified()
}

func (task *TaskStruct) SetData(data interface{}) {
	task.mu.Lock()
	defer task.mu.Unlock()
	task.Data = data
	task.MarkAsModified()
}

func (task *TaskStruct) Done(message string) {
	task.mu.Lock()
	defer task.mu.Unlock()
	task.EndTS = time.Now().Unix()
	task.Progress = 100
	if task.Message != "" {
		task.Message = message
	}
	task.MarkAsModified()
}

func (task *TaskStruct) Cancel(message string) {
	task.mu.Lock()
	defer task.mu.Unlock()
	task.EndTS = time.Now().Unix()
	if task.Message != "" {
		task.Message = message
	}
	task.MarkAsModified()
}

func (task *TaskStruct) IsExpired() bool {
	return task.EndTS > 0 && time.Now().Unix()-task.EndTS > task.T.ExpirationTime
}

func (task *TaskStruct) HasChangedSinceLastView() bool {
	return task.ModifiedSinceLastView
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

func (t *TasksData) GetAllTasksByUUID(uuid string) []*TaskStruct {
	return t.TasksMap[uuid]
}

func (t *TasksData) GetUpdatedTasksByUUID(uuid string) []*TaskStruct {
	tasks := make([]*TaskStruct, 0, 8)
	for _, task := range t.TasksMap[uuid] {
		if task.HasChangedSinceLastView() {
			tasks = append(tasks, task)
		}
	}
	return tasks
}

func (t *TasksData) GetTasksStatus_api(c echo.Context) error {
	var api getTasksStatus_api
	if err := c.Bind(&api); err != nil {
		return c.String(http.StatusBadRequest, "Invalid request")
	}
	tasks := t.GetUpdatedTasksByUUID(api.UUID)
	for _, task := range tasks {
		if task.HasChangedSinceLastView() {
			task.LastViewedTS = time.Now().Unix()
			task.ModifiedSinceLastView = false
		}
	}
	return c.JSON(http.StatusOK, tasks)
}
