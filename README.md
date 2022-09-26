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

# Usage

- 固定窗口计数器
- 滑动窗口计数器
- ✅ 漏桶算法
- ✅ 令牌桶算法

# TODO

提供集群模式，基于redis中心节点，将自己节点上的负载情况上报，从而使得某一个服务出现限流了可以将其导入到其他请求数没有那么多的机子上
