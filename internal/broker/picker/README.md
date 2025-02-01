## Picker

### 领域模型

- Resource： 指代一组资源，例如 gRPC 的后端、或者包含请求的 Queue
- Attribute： Resource 的 interface, 返回一个 keymap，不同的算法可能需要不同的 Key
- Picker： 会根据 Resource 的状态，以及请求的属性，来选择合适的 Resource。

Picker 具有以下接口 `Pick(PickInfo) PickResult`

- PickInfo： 包含请求的属性，以及 Resource 的状态
- PickResult： 包含选择的 Resource，以及选择的原因

### 数据结构

- PickerBuildInfo 提供 Picker Builder 的初始化参数，类型为 map[Resource]ResourceInfo
- ResourceInfo

| grpc 类型             | 抽象类型                      | 备注            |
|---------------------|---------------------------|---------------|
| balancer.Resource    | Resource                  | 资源            |
| resolver.Address    | ResourceInfo              | 资源信息          |
| PickerBuilder       | PickerBuilder             | picker 初始化    |
| PickerBuildInfo     | map[Resource]ResourceInfo | Build 入参      |
| balancer.Picker     | Picker                    | 选择器，提供 Pick方法 |
| balancer.PickInfo   | PickInfo                  | Pick 方法的入参    |
| balancer.PickResult | PickResult                | Pick 方法的结果    |

### 参考

```
type PickerBuilder interface {
	// Build returns a picker that will be used by gRPC to pick a Resource.
	Build(info PickerBuildInfo) Picker
}


type Picker interface {
	Pick(info PickInfo) (PickResult, error)
}
```