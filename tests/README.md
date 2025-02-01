# e2e Test

通过测试确保系统的功能和性能符合预期。
- [ ] 性能指标
- [ ] 调度器策略
  - [ ] RR
  - [ ] WRR
  - [ ] RWRR

## 性能指标
性能指标包括：
- 直接访问与 经过 proxy、Broker 的指标差异
- 指标包括 吞吐以及延迟

## 调度器策略
调度器策略指标包括：
- 调度器策略的性能指标（这部分单独测试）
- 调度器策略的负载均衡情况（通过方差来判断）

调度器策略包括：
- RR：Round Robin 随机策略，ServiceGroup 中的每个 Service 依次被选中
- WRR：Weighted Round Robin 加权随机策略，ServiceGroup 中的每个 Service 依次被选中，但是根据权重来决定被选中的概率
- RWRR：Random Weighted Round Robin 随机加权策略，ServiceGroup 中的每个 Service 依次被选中，但是根据权重来决定被选中的概率，且每次选中的 Service 都是随机的


