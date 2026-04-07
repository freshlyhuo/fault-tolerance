接口详细定义
4.1 接收配置更新接口 /fault_tolerance/health_monitor/update_config_rpc
调用方：配置中心
提供方：容错模块
接口功能：运行期间推送最新配置文件至容错模块。
请求参数 (Request Payload)：
| 字段名 | 数据类型 | 必填 | 说明 |
| :--- | :--- | :--- | :--- |
| module_name | String | 是 | 目标模块名，固定为 "fault_tolerance/health_monitor" |
| version | String | 是 | 下发的新配置文件版本号，如 "V2.1" |
| checksum | String | 是 | 配置文件的校验和CRC |
| config_data | String | 是 | 具体的阈值配置数据 |
返回参数 (Response Payload)：
| 字段名 | 数据类型 | 说明 |
| :--- | :--- | :--- |
| status_code | Integer | 更新状态：0 成功；1 校验和错误；2 解析错误；3 内部应用失败 |
| active_version| String | 当前模块实际正在使用的版本号（用于向地面确认状态） |
4.2 查询配置状态接口 /fault_tolerance/health_monitor/get_status_rpc
调用方：配置中心
提供方：容错模块
接口功能：获取容错模块当前的配置版本，用于一致性检查。
请求参数：无（或仅传 module_name）
返回参数：
| 字段名 | 数据类型 | 说明 |
| :--- | :--- | :--- |
| current_version| String | 当前生效的版本号 |
| current_checksum| String| 当前配置的校验和 |