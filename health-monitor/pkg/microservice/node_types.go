package microservice

// NodeListOptions 封装了所有可以用于 List 节点的查询参数。
type NodeListOptions struct {
	PageNum   int
	PageSize  int
	Name      string
	BasicInfo bool
}

// NodeList 是 List 方法的返回值，精确匹配 API 响应的 data 字段。
type NodeList struct {
	Total    int        `json:"total"`
	PageNum  int        `json:"pageNum"`
	PageSize int        `json:"pageSize"`
	Items    []NodeInfo `json:"list"` // 注意：Items 的类型是 NodeInfo
}

// NodeInfo 代表节点列表中的单个节点运行时信息 (basicInfo=false 时)。
type NodeInfo struct {
	ID                   string  `json:"id"`
	Address              string  `json:"address"`
	Name                 string  `json:"name"`
	Password             string  `json:"password,omitempty"` // List 的真实响应中包含 password
	Status               string  `json:"status"`
	Type                 string  `json:"type"`
	TLS                  bool    `json:"tls"`
	ContainerTotal       int     `json:"containerTotal"`
	ContainerRunning     int     `json:"containerRunning"`
	ContainerEcsmTotal   int     `json:"containerEcsmTotal"`
	ContainerEcsmRunning int     `json:"containerEcsmRunning"`
	UpTime               float64 `json:"upTime"`
	CreatedTime          string  `json:"createdTime"`
	Arch                 string  `json:"arch"`
}

// NodeDetails 代表通过 Get /node/:id 获取到的节点详细配置信息。
type NodeDetailsByID struct {
	ID          string `json:"id"`
	Address     string `json:"address"`
	Name        string `json:"name"`
	Password    string `json:"password"`
	TLS         bool   `json:"tls"`
	Type        string `json:"type"`
	CreatedTime string `json:"createdTime"`
	Arch        string `json:"arch"`
	EcsdVersion string `json:"ecsdVersion"` // Get 详情时特有的字段
}

// NodeStatus 描述了一个节点的实时运行时状态，精确匹配 GET /node/status API 的响应。
type NodeStatus struct {
	ID                   string        `json:"id"`
	Status               string        `json:"status"`
	MemoryTotal          int64         `json:"memoryTotal"`
	MemoryFree           int64         `json:"memoryFree"`
	DiskTotal            float64       `json:"diskTotal"`
	DiskFree             float64       `json:"diskFree"`
	CPUUsage             interface{}  `json:"cpuUsage"`			//离线节点是float64,在线节点是NodeCPUUsage
	Uptime               float64       `json:"uptime"`
	ProcessCount         int           `json:"processCount"`
	ContainerTotal       int           `json:"containerTotal"`
	ContainerRunning     int           `json:"containerRunning"`
	ContainerEcsmTotal   int           `json:"containerEcsmTotal"`
	ContainerEcsmRunning int           `json:"containerEcsmRunning"`
	Net                  []NodeNetInfo `json:"net"`
	Time                 NodeTimeInfo  `json:"time"`
}

// NodeCPUUsage 描述了节点的 CPU 使用情况。
type NodeCPUUsage struct {
	Total       float64   `json:"total"`
	Cores       []float64 `json:"cores"`
}

// NodeNetInfo 描述了节点的网络接口情况。
type NodeNetInfo struct {
	NetworkName string  `json:"networkName"`
	UpNet       float64 `json:"upNet"`
	DownNet     float64 `json:"downNet"`
}

// NodeTimeInfo 描述了节点的时区和时间信息。
type NodeTimeInfo struct {
	Current      int64   `json:"current"`
	Uptime       float64 `json:"uptime"`
	Timezone     string  `json:"timezone"`
	TimezoneName string  `json:"timezoneName"`
	Date         string  `json:"date"` // 从响应示例中补充
}

// NodeStatusResponse 是 GET /node/status 查询节点状态API 响应的 data 字段。
type NodeStatusResponse struct {
	Nodes []NodeStatus `json:"nodes"`
}