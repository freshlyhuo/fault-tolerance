package microservice

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"time"

	"io"
	"net/http"
)

////////////////////////////////////////////////////////////////////////////////////
// HTTP CLIENT（最简单形式，没有依赖 rest.Interface）
////////////////////////////////////////////////////////////////////////////////////

type SimpleHTTPClient struct {
	BaseURL string
	Client  *http.Client
}

func NewSimpleHTTPClient(base string) *SimpleHTTPClient {
	return &SimpleHTTPClient{
		BaseURL: base,
		Client:  &http.Client{Timeout: 10 * time.Second},
	}
}

////////////////////////////////////////////////////////////////////////////////////
// Fetcher — 负责调用节点相关 API
////////////////////////////////////////////////////////////////////////////////////

type Fetcher struct {
	http *SimpleHTTPClient
}

func NewFetcher(baseURL string) *Fetcher {
	return &Fetcher{
		http: NewSimpleHTTPClient(baseURL),
	}
}

////////////////////////////////////////////////////////////////////////////////////
// List: /api/v1/node（分页）
////////////////////////////////////////////////////////////////////////////////////

func (f *Fetcher) ListNode(ctx context.Context, opts NodeListOptions) (*NodeList, error) {
	u, _ := url.Parse(f.http.BaseURL + "/api/v1/node")
	q := u.Query()
	q.Set("pageNum", fmt.Sprintf("%d", opts.PageNum))
	q.Set("pageSize", fmt.Sprintf("%d", opts.PageSize))
	if opts.Name != "" {
		q.Set("name", opts.Name)
	}
	if opts.BasicInfo {
		q.Set("basicInfo", "true")
	}
	u.RawQuery = q.Encode()

	resp, err := f.http.Client.Get(u.String())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	// 解析结构：data → NodeList
	var result struct {
		Status  int                `json:"status"`
		Message string             `json:"message"`
		Data    NodeList `json:"data"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	return &result.Data, nil
}

func (f *Fetcher) ListContainerByNodePage(ctx context.Context, opts ListContainersByNodeOptions) (*ContainerList, error) {
    u, _ := url.Parse(f.http.BaseURL + "/api/v1/container/node")
    q := u.Query()

    q.Set("pageNum", fmt.Sprintf("%d", opts.PageNum))
    q.Set("pageSize", fmt.Sprintf("%d", opts.PageSize))
    
    for _, id := range opts.NodeIDs {
        q.Add("nodeIds[]", id)
    }

    u.RawQuery = q.Encode()

    // GET 请求部分保持不变
    req, err := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
    if err != nil {
        return nil, err
    }

    resp, err := f.http.Client.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    body, err := io.ReadAll(resp.Body)
    if err != nil {
        return nil, err
    }


    // 1. 定义临时结构体，将 Data 设为 interface{}
    var result struct {
        Status  int         `json:"status"`
        Message string      `json:"message"`
        Data    interface{} `json:"data"` // 可能是 List 对象，也可能是 Error 字符串
    }

    // 2. 解析外层 JSON
    if err := json.Unmarshal(body, &result); err != nil {
        // 如果连最外层格式都错了，那确实没办法
        return nil, fmt.Errorf("json decode failed: %w", err)
    }

    // 3. 检查业务状态码
    if result.Status != 200 {
        return nil, fmt.Errorf("API error (status=%d): %s", result.Status, result.Message)
    }

    // 4. 处理多态的 Data 字段
    switch v := result.Data.(type) {
    
    case string:
        // 情况 A: API 返回了字符串（虽然 status 是 200，但 data 是字符串的情况比较少见，但也得防着）
        // 或者如果 API status 不是 200 但在这里才捕获到错误信息
        return nil, fmt.Errorf("API returned data as string (unexpected): %s", v)

    case map[string]interface{}:
        // 情况 B: 正常数据 -> 转回 ContainerList 结构体
        tmpBytes, _ := json.Marshal(v)
        
        var list ContainerList
        // 注意：ContainerList 里的 Items 建议定义为 []interface{} 
        // 这样在 ListContainerByNode 那一层循环里再逐个处理，防止列表里某一个坏了导致整个列表挂掉
        if err := json.Unmarshal(tmpBytes, &list); err != nil {
             return nil, fmt.Errorf("failed to convert map to ContainerList: %w", err)
        }
        
        return &list, nil

    default:
        // 情况 C: nil 或其他类型
        if result.Data == nil {
             // 如果 data 是 null，通常返回一个空列表结构体防止空指针
             return &ContainerList{Items: nil, Total: 0}, nil
        }
        return nil, fmt.Errorf("unknown data type for ContainerList: %T", v)
    }
}

func (f *Fetcher) ListService(ctx context.Context, opts ListServicesOptions) (*ServiceList, error) {
	u, _ := url.Parse(f.http.BaseURL + "/api/v1/service")
	q := u.Query()
	q.Set("pageNum", fmt.Sprintf("%d", opts.PageNum))
	q.Set("pageSize", fmt.Sprintf("%d", opts.PageSize))
	u.RawQuery = q.Encode()
	resp, err := f.http.Client.Get(u.String())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	// 解析结构：data → NodeList
	var result struct {
		Status  int                `json:"status"`
		Message string             `json:"message"`
		Data    ServiceList		`json:"data"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	return &result.Data, nil
}

////////////////////////////////////////////////////////////////////////////////////
// ListAll: 自动翻页，直到全部节点返回
////////////////////////////////////////////////////////////////////////////////////

func (f *Fetcher) ListAllNode(ctx context.Context, opts NodeListOptions) ([]NodeInfo, error) {
	var all []NodeInfo
	opts.PageNum = 1
	if opts.PageSize == 0 {
		opts.PageSize = 50
	}

	for {
		page, err := f.ListNode(ctx, opts)
		if err != nil {
			return nil, err
		}

		all = append(all, page.Items...)

		if len(all) >= page.Total {
			break
		}
		opts.PageNum++
	}

	return all, nil
}

func (f *Fetcher) ListContainerByNode(ctx context.Context, opts ListContainersByNodeOptions) ([]ContainerInfo, error) {
    var all []ContainerInfo

    // 初始化分页
    if opts.PageNum == 0 {
        opts.PageNum = 1
    }
    if opts.PageSize == 0 {
        opts.PageSize = 50
    }

    for {
        page, err := f.ListContainerByNodePage(ctx, opts)
        if err != nil {
            return nil, err
        }

    
        for _, item := range page.Items {
            // 1. 将 interface{} (本质是 map) 重新转回 JSON 字节流
            itemBytes, err := json.Marshal(item)
            if err != nil {
                // 如果单个数据有问题，打印日志并跳过，不要 panic
                fmt.Printf("Error marshaling item: %v\n", err)
                continue
            }

            // 2. 将 JSON 字节流解析到具体的 ContainerInfo 结构体中
            var info ContainerInfo
            if err := json.Unmarshal(itemBytes, &info); err == nil {
                all = append(all, info)
            } else {
                fmt.Printf("Error unmarshaling to ContainerInfo: %v\n", err)
            }
        }
        // 检查分页结束条件
        // 注意：这里用 len(all) 可能不准，如果中间跳过了错误的 item
        // 建议加上 page.Items 为空时的判断
        if len(page.Items) == 0 || len(all) >= page.Total {
            break
        }

        opts.PageNum++
    }

    return all, nil
}

func (f *Fetcher) ListAllService(ctx context.Context, opts ListServicesOptions) ([]ProvisionListRow, error) {
	var all []ProvisionListRow
	opts.PageNum = 1
	if opts.PageSize == 0 {
		opts.PageSize = 50
	}

	for {
		page, err := f.ListService(ctx, opts)
		if err != nil {
			return nil, err
		}

		all = append(all, page.Items...)

		if len(all) >= page.Total {
			break
		}
		opts.PageNum++
	}

	return all, nil
}
////////////////////////////////////////////////////////////////////////////////////
// ListStatus: /api/v1/node/status?ids[]=...
////////////////////////////////////////////////////////////////////////////////////

func (f *Fetcher) ListNodeStatus(ctx context.Context, nodeIDs []string) ([]NodeStatus, error) {
	u, _ := url.Parse(f.http.BaseURL + "/api/v1/node/status")
	q := u.Query()
	for _, id := range nodeIDs {
		q.Add("ids[]", id)
	}
	u.RawQuery = q.Encode()

	resp, err := f.http.Client.Get(u.String())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	// data → { nodes: [...] }
	var result struct {
		Status  int                         `json:"status"`
		Message string                      `json:"message"`
		Data    NodeStatusResponse 			`json:"data"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}
	return result.Data.Nodes, nil
}

func (f *Fetcher) ListContainerStatus(ctx context.Context, containers []ContainerInfo) ([]ContainerInfo, error) {
	var results []ContainerInfo

	for _, c := range containers {
		queryID := c.TaskID
		if queryID == "" {
			fmt.Printf("Warning: TaskID is empty for container %s, trying ID instead.\n", c.ID)
			queryID = c.ID
		}

		u, _ := url.Parse(f.http.BaseURL + "/api/v1/container/" + queryID)

		req, err := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
		if err != nil {
			fmt.Printf("Error building request for %s: %v\n", c.ID, err)
			continue
		}

		resp, err := f.http.Client.Do(req)
		if err != nil {
			fmt.Printf("Request failed for %s: %v\n", c.ID, err)
			// 保留旧数据并标记状态
			c.Status = "NetworkError"
			results = append(results, c)
			continue
		}
		
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode != 200 {
			c.Status = fmt.Sprintf("HttpError_%d", resp.StatusCode)
			results = append(results, c)
			continue
		}

		// 解析结构
		var result struct {
			Status  int           `json:"status"`
			Message string        `json:"message"`
			Data    ContainerInfo `json:"data"` // 直接解析为结构体
		}

		// 尝试解析
		if err := json.Unmarshal(body, &result); err != nil {
			// 如果 API 又返回了字符串 "success" 或其他非 JSON 对象
			// 说明这个 queryID 还是不对
			fmt.Printf("Failed to unmarshal details for %s (queryID=%s). Body snippet: %s\n", c.ID, queryID, string(body))
			c.Status = "JsonDecodeError"
			results = append(results, c)
			continue
		}

		// 检查业务状态
		if result.Status != 200 {
			c.Status = fmt.Sprintf("ApiError: %s", result.Message)
			results = append(results, c)
			continue
		}

		detail := result.Data
		
		// 某些 API 可能返回详情时没有带 Name 或 NodeID，我们从列表数据中补全（如果是空的）
		if detail.ID == "" { detail.ID = c.ID }
		if detail.Name == "" { detail.Name = c.Name }
		
		results = append(results, detail)
	}

	return results, nil
}

func (f *Fetcher) ListServiceStatus(ctx context.Context, serviceIDs []string) ([]ServiceGet, error) {
	var results []ServiceGet

	for _, id := range serviceIDs {

		// 构建 URL
		u, _ := url.Parse(f.http.BaseURL + "/api/v1/service/" + id)

		// 发起 GET 请求
		req, err := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
		if err != nil {
			return nil, fmt.Errorf("failed to build request for id=%s: %w", id, err)
		}

		resp, err := f.http.Client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("request failed for id=%s: %w", id, err)
		}
		defer resp.Body.Close()

		// Read body
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("read body failed for id=%s: %w", id, err)
		}

		// HTTP code check
		if resp.StatusCode != 200 {
			return nil, fmt.Errorf("service %s request failed: status=%d body=%s",
				id, resp.StatusCode, string(body))
		}

		// 解析 JSON → ContainerInfo
		var result struct {
			Status  int           `json:"status"`
			Message string        `json:"message"`
			Data    ServiceGet `json:"data"`
		}

		if err := json.Unmarshal(body, &result); err != nil {
			return nil, fmt.Errorf("json decode failed for id=%s: %w", id, err)
		}

		// 将解析结果加入结果数组
		results = append(results, result.Data)
	}

	return results, nil
}
////////////////////////////////////////////////////////////////////////////////////
// FetchAllNodeStatus: 综合流程：
// 1）ListAll 获取所有节点 ID
// 2）ListStatus 查询所有节点实时运行状态
// 只输出节点状态
////////////////////////////////////////////////////////////////////////////////////

func (f *Fetcher) FetchAllNodeStatus(ctx context.Context) ([]NodeStatus, error) {
	// 第一步：获取所有节点
	allNodes, err := f.ListAllNode(ctx, NodeListOptions{
		PageSize: 50,
	})
	if err != nil {
		return nil, err
	}

	var ids []string
	for _, n := range allNodes {
		ids = append(ids, n.ID)
	}

	// 第二步：获取所有节点状态
	statusList, err := f.ListNodeStatus(ctx, ids)
	if err != nil {
		return nil, err
	}

	return statusList, nil
}

func (f *Fetcher) FetchAllContainerStatus(ctx context.Context) ([]ContainerInfo, error) {
	// 1. 获取所有节点
	allNodes, err := f.ListAllNode(ctx, NodeListOptions{PageSize: 50})
	if err != nil {
		return nil, fmt.Errorf("list nodes failed: %w", err)
	}
	if len(allNodes) == 0 {
		return []ContainerInfo{}, nil
	}

	var nodeIDs []string
	for _, n := range allNodes {
		nodeIDs = append(nodeIDs, n.ID)
	}

	// 2. 获取所有容器列表（这里需要确保能拿到 TaskID）
	allContainers, err := f.ListContainerByNode(ctx, ListContainersByNodeOptions{
		PageSize: 50,
		NodeIDs:  nodeIDs,
	})
	if err != nil {
		return nil, fmt.Errorf("list containers failed: %w", err)
	}
	if len(allContainers) == 0 {
		return []ContainerInfo{}, nil
	}

	// 3. 获取详细状态
	// 直接把整个列表传进去，而不是只传 ID 字符串列表
	statusList, err := f.ListContainerStatus(ctx, allContainers)
	if err != nil {
		return nil, fmt.Errorf("list container details failed: %w", err)
	}

	return statusList, nil
}

func (f *Fetcher) FetchAllServiceStatus(ctx context.Context) ([]ServiceGet, error) {
	// 第一步：获取所有服务
	allServices, err := f.ListAllService(ctx, ListServicesOptions{
		PageSize: 50,
	})
	if err != nil {
		return nil, err
	}

	var ids []string
	for _, n := range allServices {
		ids = append(ids, n.ID)
	}

	// 第二步：获取所有节点状态
	statusList, err := f.ListServiceStatus(ctx, ids)
	if err != nil {
		return nil, err
	}

	return statusList, nil
}


// GatherRawMetrics: fetcher 的统一接口
type RawMetrics struct {
	Nodes      []NodeStatus
	Containers []ContainerInfo
	Services   []ServiceGet
}

func (f *Fetcher) GatherRawMetrics(ctx context.Context) (*RawMetrics, error) {
	nodes, err := f.FetchAllNodeStatus(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetch nodes failed: %w", err)
	}

	containers, err := f.FetchAllContainerStatus(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetch containers failed: %w", err)
	}

	services, err := f.FetchAllServiceStatus(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetch services failed: %w", err)
	}

	return &RawMetrics{
		Nodes:      nodes,
		Containers: containers,
		Services:   services,
	}, nil
}

