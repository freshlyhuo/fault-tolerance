
package microservice

// --- Container Get && List Structures ---

// ContainerInfo 精确映射了 ECSM API 中 Container 对象的 JSON 结构。
// 它同时用于 List 和 Get 的响应。
type ContainerInfo struct {
	ID              string   `json:"id"`
	TaskID          string   `json:"taskId"`	//主键
	Name            string   `json:"name"`
	Status          string   `json:"status"`
	Uptime          int      `json:"uptime"`
	StartedTime     string   `json:"startedTime"`
	CreatedTime     string   `json:"createdTime"`
	TaskCreatedTime string   `json:"taskCreatedTime"`
	DeployStatus    string   `json:"deployStatus"`
	FailedMessage   *string  `json:"failedMessage"` // Can be null
	RestartCount    int      `json:"restartCnt"`
	DeployNum       int      `json:"deployNum"`
	CPUUsage        CPUUsage `json:"cpuUsage"`	
	MemoryLimit     int64    `json:"memoryLimit"`
	MemoryUsage     int64    `json:"memoryUsage"`
	MemoryMaxUsage  int64    `json:"memoryMaxUsage"`
	SizeUsage       int64    `json:"sizeUsage"`
	SizeLimit       int64    `json:"sizeLimit"`
	ServiceID       string   `json:"serviceId"`
	ServiceName     string   `json:"serviceName"`
	NodeID          string   `json:"nodeId"`
	Address         string   `json:"address"`
	NodeName        string   `json:"nodeName"`
	NodeArch        string   `json:"nodeArch"`
	ImageID         string   `json:"imageId"`
	ImageName       string   `json:"imageName"`
	ImageVersion    string   `json:"imageVersion"`
	ImageOS         string   `json:"imageOS"`
	ImageArch       string   `json:"imageArch"`
}

// CPUUsage 描述了容器的 CPU 使用情况。
type CPUUsage struct {
	Total float64   `json:"total"`
	Cores []float64 `json:"cores"`
}

// ContainerList 是 ListByService 和 ListByNode 方法的返回值。
type ContainerList struct {
	Total    int             `json:"total"`
	PageNum  int             `json:"pageNum"`
	PageSize int             `json:"pageSize"`
	Items    []ContainerInfo `json:"list"`
}

// ListContainersByServiceOptions 封装了查询服务下容器列表的参数。
type ListContainersByServiceOptions struct {
	PageNum    int      `json:"pageNum"`
	PageSize   int      `json:"pageSize"`
	ServiceIDs []string `json:"serviceIds"` // 必填
	Key        string   `json:"key,omitempty"`
}

type ListContainersByNodeOptions struct {
	PageNum  int      `json:"pageNum"`
	PageSize int      `json:"pageSize"`
	NodeIDs  []string `json:"nodeIds"` // 必填
	Key      string   `json:"key,omitempty"`
}

