<div align="center">
  <p>
      <pre style="float:center">
 _  .-')      ('-.      .-') _       ('-.                         _   .-')               .-') _    
( \( -O )    ( OO ).-. (  OO) )    _(  OO)                       ( '.( OO )_            (  OO) )   
 ,------.    / . --. / /     '._  (,------.  ,--.        ,-.-')   ,--.   ,--.)  ,-.-')  /     '._  
 |   /`. '   | \-.  \  |'--...__)  |  .---'  |  |.-')    |  |OO)  |   `.'   |   |  |OO) |'--...__) 
 |  /  | | .-'-'  |  | '--.  .--'  |  |      |  | OO )   |  |  \  |         |   |  |  \ '--.  .--' 
 |  |_.' |  \| |_.'  |    |  |    (|  '--.   |  |`-' |   |  |(_/  |  |'.'|  |   |  |(_/    |  |    
 |  .  '.'   |  .-.  |    |  |     |  .--'  (|  '---.'  ,|  |_.'  |  |   |  |  ,|  |_.'    |  |    
 |  |\  \    |  | |  |    |  |     |  `---.  |      |  (_|  |     |  |   |  | (_|  |       |  |    
 `--' '--'   `--' `--'    `--'     `------'  `------'    `--'     `--'   `--'   `--'       `--'    
  </pre>
  </p>
  <p>


[![Build Status](https://github.com/wwqdrh/ratelimit/actions/workflows/push.yml/badge.svg)](https://github.com/wwqdrh/ratelimit/actions)
[![codecov](https://codecov.io/gh/wwqdrh/ratelimit/branch/main/graph/badge.svg?token=4WB420ZAIO)](https://codecov.io/gh/wwqdrh/ratelimit)

  </p>
</div>

# Desc

如果只是简单的单机限流,你应该使用官方的令牌桶算法实现(https://pkg.go.dev/golang.org/x/time/rate)

> 下面是官方rate的示例

```go
limiter := NewLimiter(10, 1);

limit := Every(100 * time.Millisecond);
limiter := NewLimiter(limit, 1);

// 当使用 Wait 方法消费 Token 时，如果此时桶内 Token 数组不足 (小于 N)，那么 Wait 方法将会阻塞一段时间，直至 Token 满足条件。
// 可以设置 context 的 Deadline 或者 Timeout，来决定此次 Wait 的最长时间
func (lim *Limiter) Wait(ctx context.Context) (err error)
func (lim *Limiter) WaitN(ctx context.Context, n int) (err error)

// 满足则返回 true，同时从桶中消费 n 个 token。反之返回不消费 Token，false。
// 通常对应这样的线上场景，如果请求速率过快，就直接丢到某些请求。
func (lim *Limiter) Allow() bool
func (lim *Limiter) AllowN(now time.Time, n int) bool

// 无论 Token 是否充足，都会返回一个 Reservation * 对象
// 你可以调用该对象的 Delay() 方法，该方法返回了需要等待的时间。如果等待时间为 0，则说明不用等待。
// 如果不想等待，可以调用 Cancel() 方法，该方法会将 Token 归还。
func (lim *Limiter) Reserve() *Reservation
func (lim *Limiter) ReserveN(now time.Time, n int) *Reservation

r := lim.Reserve()
f !r.OK() {
    // Not allowed to act! Did you remember to set lim.burst to be > 0 ?
    return
}
time.Sleep(r.Delay())
Act() // 执行相关逻辑

SetLimit(Limit) // 改变放入 Token 的速率
SetBurst(int) // 改变 Token 桶大小
```

# Usage

- 固定窗口计数器
- 滑动窗口计数器
- ✅ 漏桶算法
- ✅ 令牌桶算法

# 分布式限流(TODO)

维护多个实例时也能让总体流量控制在一个整体的范围

## solution1

> 具体安装脚本可参考scripts/redis-cell.sh

基于redis-cell

一个rust编写的redis扩展模块

只有一条命令即可

```bash
CL.THROTTLE user123 15 30 60 1
               ▲     ▲  ▲  ▲ ▲
               |     |  |  | └───── apply 1 token (default if omitted)
               |     |  └──┴─────── 30 tokens / 60 seconds
               |     └───────────── 15 max_burst
               └─────────────────── key "user123"
```

> 将这个扩展添加动态添加到容器中比较麻烦, 最好自己打包一个新的镜像，记得将安装下gcc环境，否则无法导入so包或者去hubdocker搜现成的包

## solution2

提供集群模式，基于redis中心节点，将自己节点上的负载情况上报，从而使得某一个服务出现限流了可以将其导入到其他请求数没有那么多的机子上

<img src="./docs/ratelimit.png" />

使用redis作为中心服务器，每个任务启动后会定时的将容量上报

每个节点都会获取全量信息并在自己的内存中维护，避免每次发生容量不足时都去请求redis，使得大请求环境下降这个压力又带到redis中

根据各个节点的全量容量信息动态计算当前节点的容量

当某个节点容量不足时，说明这个节点机器更好请求处理更快(或者负载不均，但是可以考虑下负载均衡的策略是否正确)，可以主动承担更多的容量来帮助整体集群

1、直接向其他节点借，然后更新redis中的值，等其他节点同步的时候就能够调整它自己的容量，不过这有一定的延迟

