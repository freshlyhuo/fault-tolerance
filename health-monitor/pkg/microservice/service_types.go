// file: pkg/ecsm_client/clientset/service_types.go

package microservice

// --- Create Request Structures ---

type EcsImageConfig struct {
	Platform *Platform `json:"platform,omitempty"`
	Process  *Process  `json:"process,omitempty"`
	Root     *Root     `json:"root,omitempty"`
	Hostname string    `json:"hostname,omitempty"`
	Mounts   []Mount   `json:"mounts,omitempty"`
	SylixOS  *SylixOS  `json:"sylixos,omitempty"`
}

type Platform struct {
	OS   string `json:"os"`
	Arch string `json:"arch"`
}

type Process struct {
	Args []string `json:"args"`
	Env  []string `json:"env"`
	Cwd  string   `json:"cwd"`
}

type Root struct {
	Path     string `json:"path"`
	Readonly bool   `json:"readonly"`
}

type Mount struct {
	Destination string   `json:"destination"`
	Source      string   `json:"source"`
	Options     []string `json:"options"`
}

type SylixOS struct {
	Devices   []Device   `json:"devices,omitempty"`
	Resources *Resources `json:"resources"`
	Network   *Network   `json:"network"`
	Commands  []string   `json:"commands"`
}

type Device struct {
	Path   string `json:"path"`
	Access string `json:"access"`
}

type Resources struct {
	CPU          *CPU          `json:"cpu"`
	Memory       *Memory       `json:"memory"`
	Disk         *Disk         `json:"disk"`
	KernelObject *KernelObject `json:"kernelObject"`
}

type CPU struct {
	HighestPrio int `json:"highestPrio"`
	LowestPrio  int `json:"lowestPrio"`
}

type Memory struct {
	KheapLimit    int `json:"kheapLimit"`
	MemoryLimitMB int `json:"memoryLimitMB"`
}

type Disk struct {
	LimitMB int `json:"limitMB"`
}

type KernelObject struct {
	ThreadLimit        int `json:"threadLimit"`
	ThreadPoolLimit    int `json:"threadPoolLimit"`
	EventLimit         int `json:"eventLimit"`
	EventSetLimit      int `json:"eventSetLimit"`
	PartitionLimit     int `json:"partitionLimit"`
	RegionLimit        int `json:"regionLimit"`
	MsgQueueLimit      int `json:"msgQueueLimit"`
	TimerLimit         int `json:"timerLimit"`
	RMSLimit           int `json:"rmsLimit,omitempty"` // API 文档中没有，但 payload 示例中有
	ThreadVarLimit     int `json:"threadVarLimit,omitempty"`
	PosixMqueueLimit   int `json:"posixMqueueLimit,omitempty"`
	DlopenLibraryLimit int `json:"dlopenLibraryLimit,omitempty"`
	XSIIPCLimit        int `json:"xsiipcLimit,omitempty"`
	SocketLimit        int `json:"socketLimit,omitempty"`
	SRTPLimit          int `json:"srtpLimit,omitempty"`
	DeviceLimit        int `json:"deviceLimit,omitempty"`
}

type Network struct {
	FtpdEnable    bool `json:"ftpdEnable"`
	TelnetdEnable bool `json:"telnetdEnable"`
}

type ImageSpec struct {
	Ref         string          `json:"ref"`
	Action      string          `json:"action"` // "load" or "run"
	Config      *EcsImageConfig `json:"config"` // 假设我们只关心 EcsImageConfig
	VSOA        *ImageVSOA      `json:"vsoa,omitempty"`
	PullPolicy  string          `json:"pullPolicy,omitempty"`
	AutoUpgrade string          `json:"autoUpgrade,omitempty"`
}

type NodeSpec struct {
	Names []string `json:"names"`
}

type ImageVSOA struct {
	Password          string `json:"password,omitempty"`
	Port              *int   `json:"port,omitempty"`
	HealthPath        string `json:"healthPath,omitempty"`
	HealthTimeout     *int   `json:"healthTimeout,omitempty"`
	HealthRetries     *int   `json:"healthRetries,omitempty"`
	HealthStartPeriod *int   `json:"healthStartPeriod,omitempty"`
	HealthInterval    *int   `json:"healthInterval,omitempty"`
}




// ServiceGet mimics the response from the GET /service/:id endpoint.
// ServiceGet 精确匹配 GET /service/:id API 的成功响应 data。
type ServiceGet struct {
	ID                   string            `json:"id"`
	Name                 string            `json:"name"`
	Status               string            `json:"status"`
	ContainerStatusGroup []string          `json:"containerStatusGroup"`
	Healthy              bool              `json:"healthy"`
	Factor               int               `json:"factor"`
	Policy               string            `json:"policy"`
	InstanceOnline       int               `json:"instanceOnline"`
	InstanceActive       int               `json:"instanceActive"`
	CreatedTime          string            `json:"createdTime"`
	UpdatedTime          string            `json:"updatedTime"`
	Image                *ImageSpec        `json:"image"`          // <-- 复用共享类型
	Node                 *NodeSpec         `json:"node,omitempty"` // <-- 复用共享类型
	NodeList             []ServiceNodeInfo `json:"nodeList"`
}

// --- List Options and Response Structures ---
// ListServiceOptions 封装了所有可以用于 List 服务的查询参数。
type ListServicesOptions struct {
	PageNum  int    `json:"pageNum"`  // 必填
	PageSize int    `json:"pageSize"` // 必填
	Name     string `json:"name,omitempty"`
	// 注意：API 文档中的 'id' 字段名可能会引起混淆，因为它指的是镜像ID，
	// 我们在结构体中用更明确的名字 ImageID。
	ImageID string `json:"imageId,omitempty"`
	NodeID  string `json:"nodeId,omitempty"`
	Label   string `json:"label,omitempty"`
}

// ServiceList 是 List 方法的返回值，精确匹配 API 响应中的 data 字段。
type ServiceList struct {
	Total    int                `json:"total"`
	PageNum  int                `json:"pageNum"`
	PageSize int                `json:"pageSize"`
	Items    []ProvisionListRow `json:"list"` // 字段名是 "list"
}

// ProvisionListRow 代表服务列表中的单行数据。
type ProvisionListRow struct {
	ID                   string            `json:"id"`
	Name                 string            `json:"name"`
	Status               string            `json:"status"`
	UpdatedTime          string            `json:"updatedTime"`
	CreatedTime          string            `json:"createdTime"`
	ImageList            []ImageListEntry  `json:"imageList"`
	NodeList             []ServiceNodeInfo `json:"nodeList"`
	ContainerStatusGroup []string          `json:"containerStatusGroup"`
	Factor               int               `json:"factor"`
	Policy               string            `json:"policy"`
	ErrorInstances       []ErrorInstance   `json:"errorInstance"`
	InstanceOnline       int               `json:"instanceOnline"`
	DefaultLabels        []string          `json:"defaultLabels"`
	PathLabel            string            `json:"pathLabel"`
}

// ImageListEntry 是服务列表中内嵌的镜像信息。
type ImageListEntry struct {
	Name string `json:"name"`
	OS   string `json:"os"`
	Tag  string `json:"tag"`
}

// NodeListEntry 是服务列表中内嵌的节点信息。
type ServiceNodeInfo struct {
	NodeID   string `json:"nodeId"`
	NodeName string `json:"nodeName"`
	Address  string `json:"address"`
}

// ErrorInstance 描述了一个部署失败的实例。
type ErrorInstance struct {
	ContainerID string `json:"containerId"`
	NodeID      string `json:"nodeId"`
	NodeName    string `json:"nodeName"`
	Status      bool   `json:"status"` // 文档写的是string，但含义是bool，我们先用bool
	Message     string `json:"message"`
}

// --- Update Request Structures ---

// UpdateServiceRequest 定义了更新一个服务时，ECSM API 所需的 payload。
// 它与 CreateServiceRequest 非常相似，但包含了服务ID。
type UpdateServiceRequest struct {
	ID     string    `json:"id"`
	Name   string    `json:"name"`
	Image  ImageSpec `json:"image"`
	Node   NodeSpec  `json:"node"`
	Factor *int      `json:"factor,omitempty"`
	Policy string    `json:"policy,omitempty"` // "dynamic" or "static"

	// 注意：Update 的 payload 中似乎没有 prepull 字段，所以我们不在这里包含它。
}

// ControlServicesResponse 是批量操作服务返回的IDs列表
type ControlServicesResponse struct {
	IDs []string `json:"ids"`
}

// --- ACTIONS Structures ---

//RedeployRequest 是重新部署容器信息
type RedeployRequest struct {
	ID string `json:"id"`
}

//校验服务名称
type ValidateNameOptions struct {
	Name string `json:"name"`
	ID   string `json:"id,omitempty"`
}

//RollBackRequest 是需要回滚服务部署记录的信息
type RollBackRequest struct {
	ID       string `json:"id"`
	RecordID string `json:"recordId"`
}

// --- Statistics Structures ---

// ServiceStatistics 是服务的统计信息，如服务总数和服务健康数
type ServiceStatistics struct {
	Total  int `json:"total"`
	Health int `json:"health"`
}
