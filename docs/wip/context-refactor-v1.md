# Context
当前上下文系统管理策略混乱，混合使用token阈值、page数阈值作为触发，本次变更将设定清晰的 L2->L1->L0的触发条件，并设计一个能够应对各种边界条件的上下文模型

# Design
1. 通过全局配置设置最大上下文上限，这是一个硬上限，调用session.AddMessage 时必须检查加入新消息后是否会超过此上限，通过写屏障机制，当超过上限时，写入协程陷入阻塞
2. 除了最大上下文上限用绝对token数值计量，其他的 L0 ，L1，L2上下文窗口上限全部使用百分比来表示
4. L1 page设置一个软边界，message到达软边界后，将一半的L1 message放入压缩队列。设置软边界是因为L1压缩为L0过程中还不断有新的message产生，必需留有一定的缓冲边界，

## 硬上界和软上界
- HardMaxL0: 10%
- HardMaxL1: 60%
- SoftMaxL1: 0.6 * HardMaxL1
- HardMaxL2: 30%
- SoftMaxL2: 0.7 * HardMaxL2

## 最极端的case
len(L1) = 0.7 * HardMaxL1
len(L2) = 0.7 * HardMaxL2
将L2 commit 到L1
len(L1) + len(l2) = 36% + 21% = 57% < 60% = HardMaxL1



## L2 -> L1
Given: l2 tokens > SoftMaxL2
Then: 取回时间较早的一半L2，commit给L1

Given: 一轮交互结束后，全程没有触发 HardMaxL2
Then: commit到L1

## L1 -> L0 
Given: L1触发SoftMaxL1
Then: 取一半L1 + old L0进行压缩


## 难点
1. L2 -> L1 可能触发 L1 -> L0
如何保障主协程不阻塞

2. 极端case网络速度极慢，L1 -> L0 几乎不可用的情况下，还在不断有新消息产生会发生什么

### 解决思路
1. 引入状态锁机制，L1压缩期间不允许新的L2 page commit
2. 兜底策略：当L2触发 HardMaxL2时 L1仍然阻塞时，强制丢弃L1正在压缩的message